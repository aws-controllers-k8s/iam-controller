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

"""Integration tests for the IAM Group resource"""

import logging
import time

import pytest

from acktest.k8s import condition
from acktest.k8s import resource as k8s
from acktest.resources import random_suffix_name
from e2e import service_marker, CRD_GROUP, CRD_VERSION, load_resource
from e2e.common.types import GROUP_RESOURCE_PLURAL
from e2e.replacement_values import REPLACEMENT_VALUES
from e2e import group

DELETE_WAIT_AFTER_SECONDS = 10
CHECK_STATUS_WAIT_SECONDS = 10
MODIFY_WAIT_AFTER_SECONDS = 10


@pytest.fixture(scope="module")
def simple_group():
    group_name = random_suffix_name("my-simple-group", 24)

    replacements = REPLACEMENT_VALUES.copy()
    replacements['GROUP_NAME'] = group_name

    resource_data = load_resource(
        "group_simple",
        additional_replacements=replacements,
    )

    ref = k8s.CustomResourceReference(
        CRD_GROUP, CRD_VERSION, GROUP_RESOURCE_PLURAL,
        group_name, namespace="default",
    )
    k8s.create_custom_resource(ref, resource_data)
    cr = k8s.wait_resource_consumed_by_controller(ref)

    group.wait_until_exists(group_name)

    assert cr is not None
    assert k8s.get_resource_exists(ref)

    yield (ref, cr)

    _, deleted = k8s.delete_custom_resource(
        ref,
        period_length=DELETE_WAIT_AFTER_SECONDS,
    )
    assert deleted

    group.wait_until_deleted(group_name)


@service_marker
@pytest.mark.canary
class TestGroup:
    def test_crud(self, simple_group):
        ref, res = simple_group
        group_name = ref.name

        time.sleep(CHECK_STATUS_WAIT_SECONDS)

        condition.assert_synced(ref)

        latest_policy_arns = group.get_attached_policy_arns(group_name)
        assert latest_policy_arns == []

        # Test the code paths that synchronize the attached policies for a
        # group
        policy_arns = [
            "arn:aws:iam::aws:policy/AmazonS3ReadOnlyAccess",
        ]
        new_path = "/engineering/"
        updates = {
            "spec": {
                "policies": policy_arns,
                "path": new_path,
            },
        }
        k8s.patch_custom_resource(ref, updates)
        time.sleep(MODIFY_WAIT_AFTER_SECONDS)

        latest_policy_arns = group.get_attached_policy_arns(group_name)
        assert latest_policy_arns == policy_arns

        latest_group = group.get(group_name)
        assert latest_group["Path"] == new_path

        # Attempt to add and remove inline policies from the group
        inline_doc = '''{
"Version": "2012-10-17",
"Statement": [{
"Effect": "Allow",
"Action": ["ec2:Get*"],
"Resource": ["*"]
}]
}'''
        inline_doc_2 = '''{
"Version": "2012-10-17",
"Statement": [{
"Effect": "Allow",
"Action": ["s3:Get*"],
"Resource": ["*"]
}]
}'''
        updates = {
            "spec": {
                "inlinePolicies": {
                    "ec2get": inline_doc,
                    "s3get": inline_doc_2,
                },
            },
        }
        k8s.patch_custom_resource(ref, updates)
        time.sleep(MODIFY_WAIT_AFTER_SECONDS)

        expect_inline_policies = {
            'ec2get': inline_doc,
            's3get': inline_doc_2,
        }
        cr = k8s.get_resource(ref)
        assert cr is not None
        assert 'spec' in cr
        assert 'inlinePolicies' in cr['spec']
        assert len(cr['spec']['inlinePolicies']) == 2
        assert expect_inline_policies == cr['spec']['inlinePolicies']

        latest_inline_policies = group.get_inline_policies(group_name)
        assert len(latest_inline_policies) == 2
        assert 'ec2get' in latest_inline_policies

        got_pol_doc = latest_inline_policies['ec2get']
        nospace_got_doc = "".join(c for c in got_pol_doc if not c.isspace())
        nospace_exp_doc = "".join(c for c in inline_doc if not c.isspace())
        assert nospace_exp_doc == nospace_got_doc

        got_pol_doc = latest_inline_policies['s3get']
        nospace_got_doc = "".join(c for c in got_pol_doc if not c.isspace())
        nospace_exp_doc = "".join(c for c in inline_doc_2 if not c.isspace())
        assert nospace_exp_doc == nospace_got_doc

        inline_doc_s3_get_object = '''{
"Version": "2012-10-17",
"Statement": [{
"Effect": "Allow",
"Action": ["s3:GetObject"],
"Resource": ["*"]
}]
}'''
        # update s3get policy document
        updates = {
            "spec": {
                "inlinePolicies": {
                    "ec2get": inline_doc,
                    "s3get": inline_doc_s3_get_object,
                },
            },
        }
        k8s.patch_custom_resource(ref, updates)
        time.sleep(MODIFY_WAIT_AFTER_SECONDS)

        latest_inline_policies = group.get_inline_policies(group_name)
        assert len(latest_inline_policies) == 2
        assert 's3get' in latest_inline_policies
        assert 'ec2get' in latest_inline_policies

        # expect s3get policy document to change into inlinde_doc_s3_get_object
        got_pol_doc = latest_inline_policies['s3get']
        nospace_got_doc = "".join(c for c in got_pol_doc if not c.isspace())
        nospace_exp_doc = "".join(c for c in inline_doc_s3_get_object if not c.isspace())
        assert nospace_exp_doc == nospace_got_doc

        # Remove the inline policy we just added and check the updates are
        # reflected in the IAM API
        updates = {
            "spec": {
                "inlinePolicies": None,
            },
        }
        k8s.patch_custom_resource(ref, updates)
        time.sleep(MODIFY_WAIT_AFTER_SECONDS)

        cr = k8s.get_resource(ref)
        assert cr is not None
        assert 'spec' in cr
        assert 'inlinePolicies' not in cr['spec']

        latest_inline_policies = group.get_inline_policies(group_name)
        assert len(latest_inline_policies) == 0



@pytest.fixture(scope="module")
def test_user_for_group():
    """Creates a test IAM user for group membership tests."""
    from e2e import user
    user_name = random_suffix_name("test-user-for-group", 24)
    user.create_test_user(user_name)
    user.wait_until_exists(user_name)
    yield user_name
    user.delete_test_user(user_name)


@pytest.fixture(scope="module")
def group_with_users(test_user_for_group):
    """Creates a group with a user already added."""
    group_name = random_suffix_name("group-with-users", 24)
    user_name = test_user_for_group

    replacements = REPLACEMENT_VALUES.copy()
    replacements['GROUP_NAME'] = group_name

    resource_data = load_resource(
        "group_simple",
        additional_replacements=replacements,
    )
    # Add users to the spec
    resource_data['spec']['users'] = [user_name]

    ref = k8s.CustomResourceReference(
        CRD_GROUP, CRD_VERSION, GROUP_RESOURCE_PLURAL,
        group_name, namespace="default",
    )
    k8s.create_custom_resource(ref, resource_data)
    cr = k8s.wait_resource_consumed_by_controller(ref)

    group.wait_until_exists(group_name)

    assert cr is not None
    assert k8s.get_resource_exists(ref)

    yield (ref, cr, user_name)

    _, deleted = k8s.delete_custom_resource(
        ref,
        period_length=DELETE_WAIT_AFTER_SECONDS,
    )
    assert deleted

    group.wait_until_deleted(group_name)


@service_marker
class TestGroupUsers:
    def test_create_group_with_users(self, group_with_users):
        """Test creating a group with users specified in the spec."""
        ref, res, user_name = group_with_users
        group_name = ref.name

        time.sleep(CHECK_STATUS_WAIT_SECONDS)

        condition.assert_synced(ref)

        # Verify user is in the group via AWS API
        latest_users = group.get_users(group_name)
        assert latest_users is not None
        assert user_name in latest_users

    def test_add_users_to_group(self, simple_group, test_user_for_group):
        """Test adding users to an existing group."""
        ref, res = simple_group
        group_name = ref.name
        user_name = test_user_for_group

        time.sleep(CHECK_STATUS_WAIT_SECONDS)

        # Verify group starts with no users
        latest_users = group.get_users(group_name)
        assert latest_users is not None
        assert user_name not in latest_users

        # Add user to the group
        updates = {
            "spec": {
                "users": [user_name],
            },
        }
        k8s.patch_custom_resource(ref, updates)
        time.sleep(MODIFY_WAIT_AFTER_SECONDS)

        # Verify user is now in the group
        latest_users = group.get_users(group_name)
        assert latest_users is not None
        assert user_name in latest_users

        # Verify the CR spec reflects the users
        cr = k8s.get_resource(ref)
        assert cr is not None
        assert 'spec' in cr
        assert 'users' in cr['spec']
        assert user_name in cr['spec']['users']

    def test_remove_users_from_group(self, simple_group, test_user_for_group):
        """Test removing users from a group."""
        ref, res = simple_group
        group_name = ref.name
        user_name = test_user_for_group

        time.sleep(CHECK_STATUS_WAIT_SECONDS)

        # First add a user to the group
        updates = {
            "spec": {
                "users": [user_name],
            },
        }
        k8s.patch_custom_resource(ref, updates)
        time.sleep(MODIFY_WAIT_AFTER_SECONDS)

        # Verify user is in the group
        latest_users = group.get_users(group_name)
        assert latest_users is not None
        assert user_name in latest_users

        # Remove user from the group
        updates = {
            "spec": {
                "users": [],
            },
        }
        k8s.patch_custom_resource(ref, updates)
        time.sleep(MODIFY_WAIT_AFTER_SECONDS)

        # Verify user is no longer in the group
        latest_users = group.get_users(group_name)
        assert latest_users is not None
        assert user_name not in latest_users

    def test_delete_group_with_users(self, test_user_for_group):
        """Test that deleting a group with users removes users first."""
        group_name = random_suffix_name("group-delete-test", 24)
        user_name = test_user_for_group

        replacements = REPLACEMENT_VALUES.copy()
        replacements['GROUP_NAME'] = group_name

        resource_data = load_resource(
            "group_simple",
            additional_replacements=replacements,
        )
        resource_data['spec']['users'] = [user_name]

        ref = k8s.CustomResourceReference(
            CRD_GROUP, CRD_VERSION, GROUP_RESOURCE_PLURAL,
            group_name, namespace="default",
        )
        k8s.create_custom_resource(ref, resource_data)
        cr = k8s.wait_resource_consumed_by_controller(ref)

        group.wait_until_exists(group_name)

        assert cr is not None
        assert k8s.get_resource_exists(ref)

        time.sleep(CHECK_STATUS_WAIT_SECONDS)

        # Verify user is in the group
        latest_users = group.get_users(group_name)
        assert latest_users is not None
        assert user_name in latest_users

        # Delete the group
        _, deleted = k8s.delete_custom_resource(
            ref,
            period_length=DELETE_WAIT_AFTER_SECONDS,
        )
        assert deleted

        group.wait_until_deleted(group_name)

        # Verify the group no longer exists
        assert group.get(group_name) is None
