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

"""Integration tests for the IAM RolePolicyAttachment resource"""

import datetime
import time

import pytest

from acktest.k8s import condition
from acktest.k8s import resource as k8s
from acktest.resources import random_suffix_name
from e2e import service_marker, CRD_GROUP, CRD_VERSION, load_resource
from e2e.common.types import (
    POLICY_RESOURCE_PLURAL,
    ROLE_POLICY_ATTACHMENT_RESOURCE_PLURAL,
    ROLE_RESOURCE_PLURAL,
)
from e2e.replacement_values import REPLACEMENT_VALUES
from e2e import policy
from e2e import role

DELETE_WAIT_SECONDS = 10
CHECK_WAIT_SECONDS = 10
WAIT_TIMEOUT_SECONDS = 300
WAIT_INTERVAL_SECONDS = 10


@pytest.fixture(scope="function")
def role_and_policy():
    role_name = random_suffix_name("attach-role", 24)
    policy_name = random_suffix_name("attach-policy", 24)

    role_replacements = REPLACEMENT_VALUES.copy()
    role_replacements["ROLE_NAME"] = role_name
    role_replacements["ROLE_DESCRIPTION"] = "role for role policy attachment test"
    role_replacements["MAX_SESSION_DURATION"] = "3600"

    role_resource_data = load_resource(
        "role_simple",
        additional_replacements=role_replacements,
    )

    role_ref = k8s.CustomResourceReference(
        CRD_GROUP, CRD_VERSION, ROLE_RESOURCE_PLURAL,
        role_name, namespace="default",
    )
    k8s.create_custom_resource(role_ref, role_resource_data)
    role_cr = k8s.wait_resource_consumed_by_controller(role_ref)
    assert role_cr is not None

    policy_replacements = REPLACEMENT_VALUES.copy()
    policy_replacements["POLICY_NAME"] = policy_name
    policy_replacements["POLICY_DESCRIPTION"] = "policy for role policy attachment test"

    policy_resource_data = load_resource(
        "policy_simple",
        additional_replacements=policy_replacements,
    )

    policy_ref = k8s.CustomResourceReference(
        CRD_GROUP, CRD_VERSION, POLICY_RESOURCE_PLURAL,
        policy_name, namespace="default",
    )
    k8s.create_custom_resource(policy_ref, policy_resource_data)
    policy_cr = k8s.wait_resource_consumed_by_controller(policy_ref)
    assert policy_cr is not None

    time.sleep(CHECK_WAIT_SECONDS)
    condition.assert_synced(role_ref)
    condition.assert_synced(policy_ref)

    role.wait_until_exists(role_name)

    latest_policy_cr = k8s.get_resource(policy_ref)
    assert latest_policy_cr is not None
    assert "status" in latest_policy_cr
    assert "ackResourceMetadata" in latest_policy_cr["status"]
    assert "arn" in latest_policy_cr["status"]["ackResourceMetadata"]
    policy_arn = latest_policy_cr["status"]["ackResourceMetadata"]["arn"]

    policy.wait_until_exists(policy_arn)

    yield {
        "role_name": role_name,
        "policy_arn": policy_arn,
        "role_ref": role_ref,
        "policy_ref": policy_ref,
    }

    if k8s.get_resource_exists(role_ref):
        _, deleted = k8s.delete_custom_resource(
            role_ref,
            period_length=DELETE_WAIT_SECONDS,
        )
        assert deleted
        role.wait_until_deleted(role_name)

    if k8s.get_resource_exists(policy_ref):
        _, deleted = k8s.delete_custom_resource(
            policy_ref,
            period_length=DELETE_WAIT_SECONDS,
        )
        assert deleted
        policy.wait_until_deleted(policy_arn)


def wait_until_policy_attached(role_name: str, policy_arn: str) -> None:
    now = datetime.datetime.now()
    timeout = now + datetime.timedelta(seconds=WAIT_TIMEOUT_SECONDS)

    while True:
        if datetime.datetime.now() >= timeout:
            pytest.fail("Timed out waiting for policy to be attached to role")

        attached_policy_arns = role.get_attached_policy_arns(role_name)
        if attached_policy_arns is not None and policy_arn in attached_policy_arns:
            return

        time.sleep(WAIT_INTERVAL_SECONDS)


def wait_until_policy_detached(role_name: str, policy_arn: str) -> None:
    now = datetime.datetime.now()
    timeout = now + datetime.timedelta(seconds=WAIT_TIMEOUT_SECONDS)

    while True:
        if datetime.datetime.now() >= timeout:
            pytest.fail("Timed out waiting for policy to be detached from role")

        attached_policy_arns = role.get_attached_policy_arns(role_name)
        if attached_policy_arns is not None and policy_arn not in attached_policy_arns:
            return

        time.sleep(WAIT_INTERVAL_SECONDS)


@service_marker
@pytest.mark.canary
class TestRolePolicyAttachment:
    def test_create_delete_with_references(self, role_and_policy):
        """Test attachment lifecycle using ACK resource references."""
        role_name = role_and_policy["role_name"]
        policy_arn = role_and_policy["policy_arn"]

        attachment_name = random_suffix_name("attach", 24)
        attachment_replacements = REPLACEMENT_VALUES.copy()
        attachment_replacements["ATTACHMENT_NAME"] = attachment_name
        attachment_replacements["ROLE_CR_NAME"] = role_name
        attachment_replacements["POLICY_CR_NAME"] = role_and_policy["policy_ref"].name

        attachment_resource_data = load_resource(
            "role_policy_attachment_referring",
            additional_replacements=attachment_replacements,
        )

        attachment_ref = k8s.CustomResourceReference(
            CRD_GROUP, CRD_VERSION, ROLE_POLICY_ATTACHMENT_RESOURCE_PLURAL,
            attachment_name, namespace="default",
        )
        k8s.create_custom_resource(attachment_ref, attachment_resource_data)
        attachment_cr = k8s.wait_resource_consumed_by_controller(attachment_ref)

        assert attachment_cr is not None
        condition.assert_synced(attachment_ref)
        wait_until_policy_attached(role_name, policy_arn)

        _, deleted = k8s.delete_custom_resource(
            attachment_ref,
            period_length=DELETE_WAIT_SECONDS,
        )
        assert deleted

        wait_until_policy_detached(role_name, policy_arn)

    def test_attach_multiple_policies_to_roles(self, role_and_policy):
        """Test attaching multiple policies to multiple ACK-managed roles."""
        role1_name = role_and_policy["role_name"]
        policy1_arn = role_and_policy["policy_arn"]

        # Create second role and policy
        role2_name = random_suffix_name("attach-role2", 24)
        policy2_name = random_suffix_name("attach-policy2", 24)

        role2_replacements = REPLACEMENT_VALUES.copy()
        role2_replacements["ROLE_NAME"] = role2_name
        role2_replacements["ROLE_DESCRIPTION"] = "second role for policy attachment test"
        role2_replacements["MAX_SESSION_DURATION"] = "3600"

        role2_resource_data = load_resource(
            "role_simple",
            additional_replacements=role2_replacements,
        )

        role2_ref = k8s.CustomResourceReference(
            CRD_GROUP, CRD_VERSION, ROLE_RESOURCE_PLURAL,
            role2_name, namespace="default",
        )
        k8s.create_custom_resource(role2_ref, role2_resource_data)
        role2_cr = k8s.wait_resource_consumed_by_controller(role2_ref)
        assert role2_cr is not None

        policy2_replacements = REPLACEMENT_VALUES.copy()
        policy2_replacements["POLICY_NAME"] = policy2_name
        policy2_replacements["POLICY_DESCRIPTION"] = "second policy for attachment test"

        policy2_resource_data = load_resource(
            "policy_simple",
            additional_replacements=policy2_replacements,
        )

        policy2_ref = k8s.CustomResourceReference(
            CRD_GROUP, CRD_VERSION, POLICY_RESOURCE_PLURAL,
            policy2_name, namespace="default",
        )
        k8s.create_custom_resource(policy2_ref, policy2_resource_data)
        policy2_cr = k8s.wait_resource_consumed_by_controller(policy2_ref)
        assert policy2_cr is not None

        time.sleep(CHECK_WAIT_SECONDS)
        condition.assert_synced(role2_ref)
        condition.assert_synced(policy2_ref)

        role.wait_until_exists(role2_name)

        latest_policy2_cr = k8s.get_resource(policy2_ref)
        assert latest_policy2_cr is not None
        policy2_arn = latest_policy2_cr["status"]["ackResourceMetadata"]["arn"]
        policy.wait_until_exists(policy2_arn)

        # Attachment 1: policy1 to role1
        attachment1_name = random_suffix_name("attach1", 24)
        attachment1_replacements = REPLACEMENT_VALUES.copy()
        attachment1_replacements["ATTACHMENT_NAME"] = attachment1_name
        attachment1_replacements["ROLE_CR_NAME"] = role_and_policy["role_ref"].name
        attachment1_replacements["POLICY_CR_NAME"] = role_and_policy["policy_ref"].name

        attachment1_resource_data = load_resource(
            "role_policy_attachment_referring",
            additional_replacements=attachment1_replacements,
        )

        attachment1_ref = k8s.CustomResourceReference(
            CRD_GROUP, CRD_VERSION, ROLE_POLICY_ATTACHMENT_RESOURCE_PLURAL,
            attachment1_name, namespace="default",
        )
        k8s.create_custom_resource(attachment1_ref, attachment1_resource_data)
        attachment1_cr = k8s.wait_resource_consumed_by_controller(attachment1_ref)
        assert attachment1_cr is not None
        condition.assert_synced(attachment1_ref)
        wait_until_policy_attached(role_and_policy["role_name"], policy1_arn)

        # Attachment 2: policy2 to role2
        attachment2_name = random_suffix_name("attach2", 24)
        attachment2_replacements = REPLACEMENT_VALUES.copy()
        attachment2_replacements["ATTACHMENT_NAME"] = attachment2_name
        attachment2_replacements["ROLE_CR_NAME"] = role2_ref.name
        attachment2_replacements["POLICY_CR_NAME"] = policy2_ref.name

        attachment2_resource_data = load_resource(
            "role_policy_attachment_referring",
            additional_replacements=attachment2_replacements,
        )

        attachment2_ref = k8s.CustomResourceReference(
            CRD_GROUP, CRD_VERSION, ROLE_POLICY_ATTACHMENT_RESOURCE_PLURAL,
            attachment2_name, namespace="default",
        )
        k8s.create_custom_resource(attachment2_ref, attachment2_resource_data)
        attachment2_cr = k8s.wait_resource_consumed_by_controller(attachment2_ref)
        assert attachment2_cr is not None
        condition.assert_synced(attachment2_ref)
        wait_until_policy_attached(role2_name, policy2_arn)

        # Verify both attachments are in synced state
        condition.assert_synced(attachment1_ref)
        condition.assert_synced(attachment2_ref)

        # Clean up attachment2 and role2
        _, deleted = k8s.delete_custom_resource(
            attachment2_ref,
            period_length=DELETE_WAIT_SECONDS,
        )
        assert deleted
        wait_until_policy_detached(role2_name, policy2_arn)

        _, deleted = k8s.delete_custom_resource(
            role2_ref,
            period_length=DELETE_WAIT_SECONDS,
        )
        assert deleted
        role.wait_until_deleted(role2_name)

        _, deleted = k8s.delete_custom_resource(
            policy2_ref,
            period_length=DELETE_WAIT_SECONDS,
        )
        assert deleted
        policy.wait_until_deleted(policy2_arn)

        # Clean up attachment1
        _, deleted = k8s.delete_custom_resource(
            attachment1_ref,
            period_length=DELETE_WAIT_SECONDS,
        )
        assert deleted
        wait_until_policy_detached(role_and_policy["role_name"], policy1_arn)
