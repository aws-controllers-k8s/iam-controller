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

"""Integration tests for the additive managed-policy reconciliation mode on
the IAM Role resource.

A Role annotated with

    iam.services.k8s.aws/policy-reconciliation-mode: additive

attaches the managed policies listed in Spec.Policies but never detaches
managed policies that were attached out-of-band (for example by a
RolePolicyAttachment resource, or by a role that is shared across clusters or
accounts). The default (annotation absent) behavior remains authoritative and
is exercised by test_role.py.
"""

import time

import boto3
import pytest

from acktest.k8s import condition
from acktest.k8s import resource as k8s
from acktest.resources import random_suffix_name
from e2e import service_marker, CRD_GROUP, CRD_VERSION, load_resource
from e2e.common.types import ROLE_RESOURCE_PLURAL
from e2e.replacement_values import REPLACEMENT_VALUES
from e2e import role

DELETE_WAIT_AFTER_SECONDS = 10
CHECK_STATUS_WAIT_SECONDS = 10
MODIFY_WAIT_AFTER_SECONDS = 10
MAX_SESS_DURATION = 3600

POLICY_RECONCILE_MODE_ANNOTATION = "iam.services.k8s.aws/policy-reconciliation-mode"

# Two distinct AWS-managed policies used to exercise attach/detach behavior.
POLICY_ARN_DESIRED = "arn:aws:iam::aws:policy/AmazonS3ReadOnlyAccess"
POLICY_ARN_OUT_OF_BAND = "arn:aws:iam::aws:policy/AmazonEC2ReadOnlyAccess"
POLICY_ARN_EXTRA_DESIRED = "arn:aws:iam::aws:policy/AmazonDynamoDBReadOnlyAccess"


@pytest.fixture(scope="function")
def additive_role():
    role_name = random_suffix_name("additive-role", 24)

    replacements = REPLACEMENT_VALUES.copy()
    replacements["ROLE_NAME"] = role_name
    replacements["ROLE_DESCRIPTION"] = "role for additive policy mode test"
    replacements["MAX_SESSION_DURATION"] = str(MAX_SESS_DURATION)

    resource_data = load_resource(
        "role_simple",
        additional_replacements=replacements,
    )

    # Opt the Role into additive managed-policy reconciliation and seed it with
    # a single desired managed policy.
    metadata = resource_data.setdefault("metadata", {})
    annotations = metadata.setdefault("annotations", {})
    annotations[POLICY_RECONCILE_MODE_ANNOTATION] = "additive"
    resource_data.setdefault("spec", {})["policies"] = [POLICY_ARN_DESIRED]

    ref = k8s.CustomResourceReference(
        CRD_GROUP, CRD_VERSION, ROLE_RESOURCE_PLURAL,
        role_name, namespace="default",
    )
    k8s.create_custom_resource(ref, resource_data)
    cr = k8s.wait_resource_consumed_by_controller(ref)
    assert cr is not None

    role.wait_until_exists(role_name)
    time.sleep(CHECK_STATUS_WAIT_SECONDS)
    condition.assert_synced(ref)

    yield (ref, role_name)

    _, deleted = k8s.delete_custom_resource(
        ref,
        period_length=DELETE_WAIT_AFTER_SECONDS,
    )
    assert deleted
    role.wait_until_deleted(role_name)


@service_marker
@pytest.mark.canary
class TestRoleAdditivePolicies:
    def test_additive_mode_does_not_detach_out_of_band_policy(self, additive_role):
        ref, role_name = additive_role
        iam = boto3.client("iam")

        # The controller should have attached the single desired policy.
        latest = role.get_attached_policy_arns(role_name)
        assert latest == [POLICY_ARN_DESIRED]

        # Simulate an out-of-band attachment (e.g. a RolePolicyAttachment
        # resource or a peer cluster/account) by attaching a second managed
        # policy directly through the IAM API.
        iam.attach_role_policy(
            RoleName=role_name,
            PolicyArn=POLICY_ARN_OUT_OF_BAND,
        )

        # Give the controller several reconcile loops. In authoritative mode it
        # would detach POLICY_ARN_OUT_OF_BAND here; in additive mode it must
        # leave it in place and must not enter a perpetual drift/requeue loop.
        time.sleep(MODIFY_WAIT_AFTER_SECONDS * 3)

        latest = set(role.get_attached_policy_arns(role_name))
        assert POLICY_ARN_DESIRED in latest
        assert POLICY_ARN_OUT_OF_BAND in latest, (
            "additive mode must not detach out-of-band managed policies"
        )
        # The resource should still report as synced (no endless drift).
        condition.assert_synced(ref)

    def test_additive_mode_still_attaches_new_desired_policies(self, additive_role):
        ref, role_name = additive_role
        iam = boto3.client("iam")

        # Attach an out-of-band policy that is not part of Spec.Policies.
        iam.attach_role_policy(
            RoleName=role_name,
            PolicyArn=POLICY_ARN_OUT_OF_BAND,
        )
        time.sleep(MODIFY_WAIT_AFTER_SECONDS)

        # Add a second desired policy to Spec.Policies. The controller must
        # attach it while still leaving the out-of-band policy untouched.
        updates = {
            "spec": {
                "policies": [POLICY_ARN_DESIRED, POLICY_ARN_EXTRA_DESIRED],
            },
        }
        k8s.patch_custom_resource(ref, updates)
        time.sleep(MODIFY_WAIT_AFTER_SECONDS * 2)
        condition.assert_synced(ref)

        latest = set(role.get_attached_policy_arns(role_name))
        assert POLICY_ARN_DESIRED in latest
        assert POLICY_ARN_EXTRA_DESIRED in latest
        assert POLICY_ARN_OUT_OF_BAND in latest

        # Removing a policy from Spec.Policies must NOT detach it in additive
        # mode: the desired set only drives attaches, never detaches.
        updates = {
            "spec": {
                "policies": [POLICY_ARN_EXTRA_DESIRED],
            },
        }
        k8s.patch_custom_resource(ref, updates)
        time.sleep(MODIFY_WAIT_AFTER_SECONDS * 2)
        condition.assert_synced(ref)

        latest = set(role.get_attached_policy_arns(role_name))
        assert POLICY_ARN_EXTRA_DESIRED in latest
        assert POLICY_ARN_DESIRED in latest, (
            "additive mode must not detach a policy removed from Spec.Policies"
        )
        assert POLICY_ARN_OUT_OF_BAND in latest
