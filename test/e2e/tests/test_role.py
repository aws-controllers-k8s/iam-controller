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

"""Integration tests for the IAM Role resource"""

import logging
import json
import time

import pytest

from acktest.k8s import condition
from acktest.k8s import resource as k8s
from acktest.resources import random_suffix_name
from e2e import service_marker, CRD_GROUP, CRD_VERSION, load_resource
from e2e.common.types import ROLE_RESOURCE_PLURAL
from e2e.replacement_values import REPLACEMENT_VALUES
from e2e import role
from e2e import tag

DELETE_WAIT_AFTER_SECONDS = 10
CHECK_STATUS_WAIT_SECONDS = 10
MODIFY_WAIT_AFTER_SECONDS = 10
MAX_SESS_DURATION = 3600 # Note: minimum of 3600 seconds...


@pytest.fixture(scope="module")
def simple_role():
    role_name = random_suffix_name("my-simple-role", 24)
    role_desc = "a simple role"

    replacements = REPLACEMENT_VALUES.copy()
    replacements['ROLE_NAME'] = role_name
    replacements['ROLE_DESCRIPTION'] = role_desc
    replacements['MAX_SESSION_DURATION'] = str(MAX_SESS_DURATION)

    resource_data = load_resource(
        "role_simple",
        additional_replacements=replacements,
    )

    ref = k8s.CustomResourceReference(
        CRD_GROUP, CRD_VERSION, ROLE_RESOURCE_PLURAL,
        role_name, namespace="default",
    )
    k8s.create_custom_resource(ref, resource_data)
    cr = k8s.wait_resource_consumed_by_controller(ref)

    role.wait_until_exists(role_name)

    assert cr is not None
    assert k8s.get_resource_exists(ref)

    yield (ref, cr)

    _, deleted = k8s.delete_custom_resource(
        ref,
        period_length=DELETE_WAIT_AFTER_SECONDS,
    )
    assert deleted

    role.wait_until_deleted(role_name)


@service_marker
@pytest.mark.canary
class TestRole:
    def test_crud(self, simple_role):
        ref, res = simple_role
        role_name = ref.name

        time.sleep(CHECK_STATUS_WAIT_SECONDS)

        condition.assert_synced(ref)

        # Before we update the Role CR below, let's check to see that the
        # MaxSessionDuration field in the CR is still what we set in the
        # original Create call.
        cr = k8s.get_resource(ref)
        assert cr is not None
        assert 'spec' in cr
        assert 'maxSessionDuration' in cr['spec']
        assert cr['spec']['maxSessionDuration'] == MAX_SESS_DURATION

        condition.assert_synced(ref)

        latest = role.get(role_name)
        assert latest is not None
        assert latest['MaxSessionDuration'] == MAX_SESS_DURATION

        new_max_sess_duration = MAX_SESS_DURATION + 100

        # We're now going to modify the MaxSessionDuration field of the Role,
        # wait some time and verify that the IAM server-side resource
        # shows the new value of the field.
        updates = {
            "spec": {"maxSessionDuration": new_max_sess_duration},
        }
        k8s.patch_custom_resource(ref, updates)
        time.sleep(MODIFY_WAIT_AFTER_SECONDS)

        latest = role.get(role_name)
        assert latest is not None
        assert latest['MaxSessionDuration'] == new_max_sess_duration

        # Test the code paths that synchronize the attached policies for a role
        policy_arns = [
            "arn:aws:iam::aws:policy/AmazonS3ReadOnlyAccess",
        ]
        permissionsBoundary = 'arn:aws:iam::aws:policy/AmazonS3ReadOnlyAccess'
        updates = {
            "spec": {
                "policies": policy_arns,
                "permissionsBoundary": permissionsBoundary,
            },
        }
        k8s.patch_custom_resource(ref, updates)
        time.sleep(MODIFY_WAIT_AFTER_SECONDS)

        latest_policy_arns = role.get_attached_policy_arns(role_name)
        assert latest_policy_arns == policy_arns

        latest_role = role.get(role_name)
        assert latest_role["PermissionsBoundary"]["PermissionsBoundaryArn"] == permissionsBoundary

        # Same update code path check for tags...
        latest_tags = role.get_tags(role_name)
        before_update_expected_tags = [
            {
                "Key": "tag1",
                "Value": "val1"
            }
        ]
        assert tag.cleaned(latest_tags) == before_update_expected_tags
        new_tags = [
            {
                "key": "tag2",
                "value": "val2",
            }
        ]
        updates = {
            "spec": {"tags": new_tags},
        }
        k8s.patch_custom_resource(ref, updates)
        time.sleep(MODIFY_WAIT_AFTER_SECONDS)

        after_update_expected_tags = [
            {
                "Key": "tag2",
                "Value": "val2",
            }
        ]
        latest_tags = role.get_tags(role_name)
        assert tag.cleaned(latest_tags) == after_update_expected_tags

        # Attempt to add and remove inline policies from the role
        inline_doc = '''{
"Version": "2012-10-17",
"Statement": [{
"Effect": "Allow",
"Action": ["ec2:Get*"],
"Resource": ["*"]
}]
}'''
        updates = {
            "spec": {
                "inlinePolicies": {
                    "ec2get": inline_doc,
                },
            },
        }
        k8s.patch_custom_resource(ref, updates)
        time.sleep(MODIFY_WAIT_AFTER_SECONDS)

        expect_inline_policies = {
            'ec2get': inline_doc,
        }
        cr = k8s.get_resource(ref)
        assert cr is not None
        assert 'spec' in cr
        assert 'inlinePolicies' in cr['spec']
        assert len(cr['spec']['inlinePolicies']) == 1
        assert expect_inline_policies == cr['spec']['inlinePolicies']

        latest_inline_policies = role.get_inline_policies(role_name)
        assert len(latest_inline_policies) == 1
        assert 'ec2get' in latest_inline_policies
        got_pol_doc = latest_inline_policies['ec2get']
        nospace_got_doc = "".join(c for c in got_pol_doc if not c.isspace())
        nospace_exp_doc = "".join(c for c in inline_doc if not c.isspace())

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
        assert "spec" in cr
        assert "inlinePolicies" not in cr["spec"]

        latest_inline_policies = role.get_inline_policies(role_name)
        assert len(latest_inline_policies) == 0

        # AssumeRolePolicyDocument tests

        assume_role_policy_doc = '''{
    "Version": "2012-10-17",
    "Statement": [
        {
            "Effect": "Allow",
            "Principal": {
                "Service": ["ec2.amazonaws.com"]
            },
            "Action": ["sts:AssumeRole"]
        }
    ]}'''

        cr = k8s.get_resource(ref)
        assert cr is not None
        assert 'spec' in cr
        assert 'assumeRolePolicyDocument' in cr['spec']

        assume_role_policy_as_obj = json.loads(assume_role_policy_doc)
        k8s_assume_role_policy = json.loads(cr['spec']['assumeRolePolicyDocument'])
        assert assume_role_policy_as_obj == k8s_assume_role_policy

        assume_role_policy_to_deny_doc = '''{
    "Version": "2012-10-17",
    "Statement": [
        {
            "Effect": "Deny",
            "Principal": {
                "Service": ["ec2.amazonaws.com"]
            },
            "Action": ["sts:AssumeRole"]
        }
    ]}'''

        updates = {
            'spec': {
                'assumeRolePolicyDocument': assume_role_policy_to_deny_doc,
            }
        }

        k8s.patch_custom_resource(ref, updates)
        time.sleep(MODIFY_WAIT_AFTER_SECONDS)

        cr = k8s.get_resource(ref)
        assert cr is not None
        assert 'spec' in cr
        assert 'assumeRolePolicyDocument' in cr['spec']

        assume_role_policy_deny_obj = json.loads(assume_role_policy_to_deny_doc)
        k8s_assume_role_policy_deny = json.loads(cr['spec']['assumeRolePolicyDocument'])
        assert assume_role_policy_deny_obj == k8s_assume_role_policy_deny

        # AWS slightly modifies the JSON structure underneath us here, so the documents
        # are not identical. Instead, we can ensure that the change we made is reflected.
        latest_assume_role_policy_doc = role.get_assume_role_policy(role_name)
        assert latest_assume_role_policy_doc['Statement'][0]['Effect'] == k8s_assume_role_policy_deny['Statement'][0]['Effect']

        # Assume role policies cannot be entirely deleted, so CRU is tested here.
