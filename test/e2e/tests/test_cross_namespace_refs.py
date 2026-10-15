# Copyright Amazon.com Inc. or its affiliates. All Rights Reserved.
#
# Licensed under the Apache License, Version 2.0 (the "License"). You may
# not use this file except in compliance with the License. A copy of the
# License is located at
#
#	 http://aws.amazon.com/apache2.0/
#
# or in the "license" file accompanying this file. This file is distributed
# on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either
# express or implied. See the License for the specific language governing
# permissions and limitations under the License.

"""Integration tests for cross-namespace resource reference behavior.

These tests validate the --enable-cross-namespace flag behavior with an
IAM Role whose policyRefs[].from.namespace points to another namespace.

The cross-namespace deprecation notice is surfaced as an ACK.Advisory
condition carrying the reason "CrossNamespaceOptInRequired" (not as a
dedicated ACK.CrossNamespaceOptInRequired condition type).

Scenario 1 (flag=true, Phase 1 default):
  - Create an IAM Policy in one namespace
  - Create an IAM Role in a different namespace with
    policyRefs[].from.namespace pointing to the policy's namespace
  - Assert the Role reconciles successfully (policy ARN attached)
  - Assert an ACK.Advisory condition with reason CrossNamespaceOptInRequired
    is present on the Role

Scenario 2 (flag=false):
  - Redeploy the controller with --enable-cross-namespace=false
  - Create the same Role/Policy pair across namespaces
  - Assert the Role enters a terminal state with ACK.Terminal=True
  - Assert the condition message contains "enable-cross-namespace"

Scenario 3 (same-namespace):
  - Create Policy and Role in the same namespace
  - Assert the Role reconciles successfully regardless of flag value
  - Assert NO ACK.Advisory condition with reason CrossNamespaceOptInRequired

"""

import os
import time

import pytest

from acktest.k8s import condition
from acktest.k8s import resource as k8s
from acktest.resources import random_suffix_name
from e2e import service_marker, CRD_GROUP, CRD_VERSION, load_resource
from e2e.common.types import POLICY_RESOURCE_PLURAL, ROLE_RESOURCE_PLURAL
from e2e.replacement_values import REPLACEMENT_VALUES
from e2e import role
from e2e import policy

DELETE_ROLE_TIMEOUT_SECONDS = 10
DELETE_POLICY_TIMEOUT_SECONDS = 30
WAIT_AFTER_CREATE_SECONDS = 10
TERMINAL_CONDITION_WAIT_PERIODS = 10
TERMINAL_CONDITION_PERIOD_LENGTH = 15
DEPRECATION_CONDITION_WAIT_PERIODS = 10
DEPRECATION_CONDITION_PERIOD_LENGTH = 15

# The cross-namespace deprecation notice is surfaced as an ACK.Advisory
# condition carrying the reason CrossNamespaceOptInRequired, rather than as a
# dedicated condition type. See runtime/pkg/runtime/cross_namespace.go.
ADVISORY_CONDITION_TYPE = "ACK.Advisory"
CROSS_NAMESPACE_OPT_IN_REQUIRED_REASON = "CrossNamespaceOptInRequired"


def _get_cross_namespace_advisory(reference):
    """Returns the ACK.Advisory condition carrying the
    CrossNamespaceOptInRequired reason from the resource's
    .status.conditions, or None if it is not present.
    """
    cr = k8s.get_resource(reference)
    conditions = cr.get("status", {}).get("conditions", [])
    for c in conditions:
        if (
            c.get("type") == ADVISORY_CONDITION_TYPE
            and c.get("reason") == CROSS_NAMESPACE_OPT_IN_REQUIRED_REASON
        ):
            return c
    return None


@service_marker
class TestCrossNamespaceRefs:
    """Tests for cross-namespace resource reference behavior.

    These tests use two separate namespaces: one containing a Policy and
    another containing a Role that references the Policy across namespaces.

    The controller must be deployed with the appropriate --enable-cross-namespace
    flag value for each scenario. Scenario 1 uses the Phase 1 default (true),
    Scenario 2 requires redeployment with --enable-cross-namespace=false.
    """

    @pytest.mark.skipif(
        os.environ.get("ENABLE_CROSS_NAMESPACE", "true").lower() == "false",
        reason="requires controller deployed with --enable-cross-namespace=true (Phase 1 default); "
               "skipped when ENABLE_CROSS_NAMESPACE=false",
    )
    def test_cross_namespace_ref_allowed_with_deprecation_warning(self):
        """Scenario 1: When --enable-cross-namespace is true (Phase 1 default),
        a Role referencing a Policy in a different namespace should reconcile
        successfully AND emit an ACK.Advisory condition carrying the
        CrossNamespaceOptInRequired reason.

        This test requires the controller to be deployed with
        --enable-cross-namespace=true (the Phase 1 default). It validates
        that cross-namespace references work but produce a deprecation warning.

        """
        policy_ns = random_suffix_name("policy-ns", 24)
        role_ns = random_suffix_name("role-ns", 24)
        policy_name = random_suffix_name("xns-policy", 24)
        role_name = random_suffix_name("xns-role", 24)

        policy_ref = None
        role_ref = None

        try:
            # Create the two namespaces
            k8s.create_k8s_namespace(policy_ns)
            k8s.create_k8s_namespace(role_ns)
            time.sleep(WAIT_AFTER_CREATE_SECONDS)

            # Create the Policy in the policy namespace.
            # NOTE: POLICY_NAMESPACE must be set BEFORE POLICY_NAME because
            # the placeholder replacement is a naive string replace and
            # $POLICY_NAME would otherwise be substituted as a prefix of
            # $POLICY_NAMESPACE.
            replacements = REPLACEMENT_VALUES.copy()
            replacements["POLICY_NAMESPACE"] = policy_ns
            replacements["POLICY_NAME"] = policy_name
            replacements["POLICY_DESCRIPTION"] = "cross-namespace test policy"

            policy_data = load_resource(
                "policy_simple_namespace",
                additional_replacements=replacements,
            )

            policy_ref = k8s.CustomResourceReference(
                CRD_GROUP, CRD_VERSION, POLICY_RESOURCE_PLURAL,
                policy_name, namespace=policy_ns,
            )
            k8s.create_custom_resource(policy_ref, policy_data)
            cr = k8s.wait_resource_consumed_by_controller(policy_ref)
            assert cr is not None
            assert k8s.get_resource_exists(policy_ref)

            # Wait for the Policy to be created in IAM
            cr = k8s.get_resource(policy_ref)
            assert "status" in cr
            assert "ackResourceMetadata" in cr["status"]
            assert "arn" in cr["status"]["ackResourceMetadata"]
            policy_arn = cr["status"]["ackResourceMetadata"]["arn"]
            policy.wait_until_exists(policy_arn)

            # Create the Role in the role namespace referencing the
            # Policy in the policy namespace
            replacements = REPLACEMENT_VALUES.copy()
            replacements["POLICY_NAMESPACE"] = policy_ns
            replacements["POLICY_NAME"] = policy_name
            replacements["ROLE_NAME"] = role_name

            role_data = load_resource(
                "role_referring_namespace",
                additional_replacements=replacements,
            )

            role_ref = k8s.CustomResourceReference(
                CRD_GROUP, CRD_VERSION, ROLE_RESOURCE_PLURAL,
                role_name, namespace=role_ns,
            )
            k8s.create_custom_resource(role_ref, role_data)
            cr = k8s.wait_resource_consumed_by_controller(role_ref)
            assert cr is not None
            assert k8s.get_resource_exists(role_ref)

            # Wait for the Role to sync successfully
            time.sleep(WAIT_AFTER_CREATE_SECONDS)
            condition.assert_synced(role_ref)

            # Verify the Role was created in IAM
            role.wait_until_exists(role_name)

            # Verify the policy from the namespace containing the Policy is attached
            attached_arns = role.get_attached_policy_arns(role_name)
            assert attached_arns is not None, (
                "Role should have attached policies"
            )
            assert policy_arn in attached_arns, (
                f"Policy ARN {policy_arn} from policy namespace should be "
                f"attached to the role, but found: {attached_arns}"
            )

            # Verify the cross-namespace advisory is present: an ACK.Advisory
            # condition carrying the CrossNamespaceOptInRequired reason.
            assert k8s.wait_on_condition(
                role_ref,
                ADVISORY_CONDITION_TYPE,
                "True",
                wait_periods=DEPRECATION_CONDITION_WAIT_PERIODS,
                period_length=DEPRECATION_CONDITION_PERIOD_LENGTH,
            ), (
                "Expected ACK.Advisory condition to be True "
                "for cross-namespace ref when flag is enabled"
            )

            deprecation_condition = _get_cross_namespace_advisory(role_ref)
            assert deprecation_condition is not None, (
                "Expected an ACK.Advisory condition with reason "
                "CrossNamespaceOptInRequired for cross-namespace ref when "
                "flag is enabled"
            )
            assert deprecation_condition["status"] == "True"
            # Verify the deprecation message mentions the flag name
            assert "enable-cross-namespace" in deprecation_condition.get(
                "message", ""
            ), (
                "Deprecation condition message should reference the "
                "--enable-cross-namespace flag"
            )

        finally:
            # Clean up: delete Role first to avoid cascading delete issues
            if role_ref is not None and k8s.get_resource_exists(role_ref):
                _, deleted = k8s.delete_custom_resource(
                    role_ref,
                    period_length=DELETE_ROLE_TIMEOUT_SECONDS,
                )
                assert deleted
                role.wait_until_deleted(role_name)

            if policy_ref is not None and k8s.get_resource_exists(policy_ref):
                _, deleted = k8s.delete_custom_resource(
                    policy_ref,
                    period_length=DELETE_POLICY_TIMEOUT_SECONDS,
                )
                assert deleted
                policy.wait_until_deleted(policy_arn)

            # Clean up namespaces
            try:
                k8s.delete_k8s_namespace(role_ns)
            except Exception:
                pass
            try:
                k8s.delete_k8s_namespace(policy_ns)
            except Exception:
                pass

    @pytest.mark.skipif(
        os.environ.get("ENABLE_CROSS_NAMESPACE", "true").lower() != "false",
        reason="requires controller deployed with --enable-cross-namespace=false; "
               "set ENABLE_CROSS_NAMESPACE=false to run",
    )
    def test_cross_namespace_ref_rejected_when_flag_disabled(self):
        """Scenario 2: When --enable-cross-namespace is set to false,
        a Role referencing a Policy in a different namespace should enter
        a terminal state.

        This test requires the controller to be redeployed with
        --enable-cross-namespace=false (e.g., via Helm:
        --set enableCrossNamespace=false).

        """
        policy_ns = random_suffix_name("policy-ns", 24)
        role_ns = random_suffix_name("role-ns", 24)
        policy_name = random_suffix_name("xns-policy", 24)
        role_name = random_suffix_name("xns-role", 24)

        policy_ref = None
        role_ref = None

        try:
            # Create the two namespaces
            k8s.create_k8s_namespace(policy_ns)
            k8s.create_k8s_namespace(role_ns)
            time.sleep(WAIT_AFTER_CREATE_SECONDS)

            # Create the Policy in the policy namespace.
            # NOTE: POLICY_NAMESPACE must be set BEFORE POLICY_NAME because
            # the placeholder replacement is a naive string replace and
            # $POLICY_NAME would otherwise be substituted as a prefix of
            # $POLICY_NAMESPACE.
            replacements = REPLACEMENT_VALUES.copy()
            replacements["POLICY_NAMESPACE"] = policy_ns
            replacements["POLICY_NAME"] = policy_name
            replacements["POLICY_DESCRIPTION"] = "cross-namespace test policy"

            policy_data = load_resource(
                "policy_simple_namespace",
                additional_replacements=replacements,
            )

            policy_ref = k8s.CustomResourceReference(
                CRD_GROUP, CRD_VERSION, POLICY_RESOURCE_PLURAL,
                policy_name, namespace=policy_ns,
            )
            k8s.create_custom_resource(policy_ref, policy_data)
            cr = k8s.wait_resource_consumed_by_controller(policy_ref)
            assert cr is not None
            assert k8s.get_resource_exists(policy_ref)

            # Wait for the Policy to be created in IAM
            cr = k8s.get_resource(policy_ref)
            assert "status" in cr
            assert "ackResourceMetadata" in cr["status"]
            assert "arn" in cr["status"]["ackResourceMetadata"]
            policy_arn = cr["status"]["ackResourceMetadata"]["arn"]
            policy.wait_until_exists(policy_arn)

            # Create the Role in the role namespace referencing the
            # Policy in the policy namespace
            replacements = REPLACEMENT_VALUES.copy()
            replacements["POLICY_NAMESPACE"] = policy_ns
            replacements["POLICY_NAME"] = policy_name
            replacements["ROLE_NAME"] = role_name

            role_data = load_resource(
                "role_referring_namespace",
                additional_replacements=replacements,
            )

            role_ref = k8s.CustomResourceReference(
                CRD_GROUP, CRD_VERSION, ROLE_RESOURCE_PLURAL,
                role_name, namespace=role_ns,
            )
            k8s.create_custom_resource(role_ref, role_data)
            cr = k8s.wait_resource_consumed_by_controller(role_ref)
            assert cr is not None
            assert k8s.get_resource_exists(role_ref)

            # Wait for the controller to process the Role and set the
            # terminal condition
            assert k8s.wait_on_condition(
                role_ref,
                "ACK.Terminal",
                "True",
                wait_periods=TERMINAL_CONDITION_WAIT_PERIODS,
                period_length=TERMINAL_CONDITION_PERIOD_LENGTH,
            ), "Expected ACK.Terminal condition to be True for cross-namespace ref"

            # Verify the terminal condition message contains the flag name
            terminal_condition = k8s.get_resource_condition(
                role_ref, "ACK.Terminal",
            )
            assert terminal_condition is not None
            assert terminal_condition["status"] == "True"
            assert "enable-cross-namespace" in terminal_condition.get(
                "message", ""
            ), (
                "Terminal condition message should reference the "
                "--enable-cross-namespace flag"
            )

            # Verify the Role was NOT created in IAM
            assert role.get(role_name) is None, (
                "Role should not exist in IAM when cross-namespace ref is rejected"
            )

        finally:
            # Clean up resources in reverse order
            if role_ref is not None and k8s.get_resource_exists(role_ref):
                _, deleted = k8s.delete_custom_resource(
                    role_ref,
                    period_length=DELETE_ROLE_TIMEOUT_SECONDS,
                )
                assert deleted

            if policy_ref is not None and k8s.get_resource_exists(policy_ref):
                _, deleted = k8s.delete_custom_resource(
                    policy_ref,
                    period_length=DELETE_POLICY_TIMEOUT_SECONDS,
                )
                assert deleted
                policy.wait_until_deleted(policy_arn)

            # Clean up namespaces
            try:
                k8s.delete_k8s_namespace(role_ns)
            except Exception:
                pass
            try:
                k8s.delete_k8s_namespace(policy_ns)
            except Exception:
                pass

    def test_same_namespace_ref_always_allowed(self):
        """Scenario 3: A Role referencing a Policy in the same namespace
        should always reconcile successfully regardless of the
        --enable-cross-namespace flag value.

        This test validates that same-namespace references are never
        affected by the cross-namespace flag. It should pass with both
        --enable-cross-namespace=true and --enable-cross-namespace=false.

        """
        test_ns = random_suffix_name("same-ns", 24)
        policy_name = random_suffix_name("sns-policy", 24)
        role_name = random_suffix_name("sns-role", 24)

        policy_ref = None
        role_ref = None

        try:
            # Create the test namespace
            k8s.create_k8s_namespace(test_ns)
            time.sleep(WAIT_AFTER_CREATE_SECONDS)

            # Create the Policy in the test namespace.
            # NOTE: POLICY_NAMESPACE must be set BEFORE POLICY_NAME because
            # the placeholder replacement is a naive string replace and
            # $POLICY_NAME would otherwise be substituted as a prefix of
            # $POLICY_NAMESPACE.
            replacements = REPLACEMENT_VALUES.copy()
            replacements["POLICY_NAMESPACE"] = test_ns
            replacements["POLICY_NAME"] = policy_name
            replacements["POLICY_DESCRIPTION"] = "same-namespace test policy"

            policy_data = load_resource(
                "policy_simple_namespace",
                additional_replacements=replacements,
            )

            policy_ref = k8s.CustomResourceReference(
                CRD_GROUP, CRD_VERSION, POLICY_RESOURCE_PLURAL,
                policy_name, namespace=test_ns,
            )
            k8s.create_custom_resource(policy_ref, policy_data)
            cr = k8s.wait_resource_consumed_by_controller(policy_ref)
            assert cr is not None
            assert k8s.get_resource_exists(policy_ref)

            # Wait for the Policy to be created in IAM
            cr = k8s.get_resource(policy_ref)
            assert "status" in cr
            assert "ackResourceMetadata" in cr["status"]
            assert "arn" in cr["status"]["ackResourceMetadata"]
            policy_arn = cr["status"]["ackResourceMetadata"]["arn"]
            policy.wait_until_exists(policy_arn)

            # Create the Role in the SAME namespace referencing the Policy
            replacements = REPLACEMENT_VALUES.copy()
            replacements["POLICY_NAMESPACE"] = test_ns
            replacements["POLICY_NAME"] = policy_name
            replacements["ROLE_NAME"] = role_name

            role_data = load_resource(
                "role_referring_namespace",
                additional_replacements=replacements,
            )

            role_ref = k8s.CustomResourceReference(
                CRD_GROUP, CRD_VERSION, ROLE_RESOURCE_PLURAL,
                role_name, namespace=test_ns,
            )
            k8s.create_custom_resource(role_ref, role_data)
            cr = k8s.wait_resource_consumed_by_controller(role_ref)
            assert cr is not None
            assert k8s.get_resource_exists(role_ref)

            # Wait for the Role to sync successfully
            time.sleep(WAIT_AFTER_CREATE_SECONDS)
            condition.assert_synced(role_ref)

            # Verify the Role was created in IAM
            role.wait_until_exists(role_name)

            # Verify the policy is attached
            attached_arns = role.get_attached_policy_arns(role_name)
            assert attached_arns is not None, (
                "Role should have attached policies"
            )
            assert policy_arn in attached_arns, (
                f"Policy ARN {policy_arn} should be attached to the role, "
                f"but found: {attached_arns}"
            )

            # Verify NO cross-namespace advisory is present: there must be no
            # ACK.Advisory condition carrying the CrossNamespaceOptInRequired
            # reason (same-namespace refs never trigger the deprecation notice)
            cr = k8s.get_resource(role_ref)
            conditions = cr.get("status", {}).get("conditions", [])
            deprecation_conditions = [
                c for c in conditions
                if c.get("type") == ADVISORY_CONDITION_TYPE
                and c.get("reason") == CROSS_NAMESPACE_OPT_IN_REQUIRED_REASON
            ]
            assert len(deprecation_conditions) == 0, (
                "Same-namespace reference should NOT have an ACK.Advisory "
                "condition with reason CrossNamespaceOptInRequired, but "
                f"found: {deprecation_conditions}"
            )

            # Also verify no terminal condition (sanity check)
            terminal_conditions = [
                c for c in conditions
                if c.get("type") == "ACK.Terminal"
                and c.get("status") == "True"
            ]
            assert len(terminal_conditions) == 0, (
                "Same-namespace reference should NOT have ACK.Terminal "
                f"condition, but found: {terminal_conditions}"
            )

        finally:
            # Clean up: delete Role first to avoid cascading delete issues
            if role_ref is not None and k8s.get_resource_exists(role_ref):
                _, deleted = k8s.delete_custom_resource(
                    role_ref,
                    period_length=DELETE_ROLE_TIMEOUT_SECONDS,
                )
                assert deleted
                role.wait_until_deleted(role_name)

            if policy_ref is not None and k8s.get_resource_exists(policy_ref):
                _, deleted = k8s.delete_custom_resource(
                    policy_ref,
                    period_length=DELETE_POLICY_TIMEOUT_SECONDS,
                )
                assert deleted
                policy.wait_until_deleted(policy_arn)

            # Clean up namespace
            try:
                k8s.delete_k8s_namespace(test_ns)
            except Exception:
                pass
