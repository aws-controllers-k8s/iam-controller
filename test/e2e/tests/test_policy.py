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

"""Integration tests for the IAM Policy resource"""

import json
import time

import pytest

from enum import Enum
from acktest.k8s import condition
from acktest.k8s import resource as k8s
from acktest.resources import random_suffix_name
from acktest.adoption import ADOPT_ADOPTION_POLICY, ADOPT_OR_CREATE_ADOPTION_POLICY
from e2e import service_marker, CRD_GROUP, CRD_VERSION, load_resource
from e2e.bootstrap_resources import get_bootstrap_resources
from e2e.common.types import POLICY_RESOURCE_PLURAL
from e2e.replacement_values import REPLACEMENT_VALUES
from e2e import policy
from e2e import tag

DELETE_WAIT_AFTER_SECONDS = 10
CHECK_WAIT_AFTER_SECONDS = 10
# NOTE(jaypipes): I've seen Tagris take nearly 20 seconds to return updated tag
# information on a resource.
MODIFY_WAIT_AFTER_SECONDS = 20
CREATE_WAIT_AFTER_SECONDS = 10

@pytest.fixture(scope="module")
def simple_policy():
    policy_name = random_suffix_name("my-simple-policy", 24)
    policy_desc = "a simple policy"

    replacements = REPLACEMENT_VALUES.copy()
    replacements['POLICY_NAME'] = policy_name
    replacements['POLICY_DESCRIPTION'] = policy_desc

    resource_data = load_resource(
        "policy_simple",
        additional_replacements=replacements,
    )

    ref = k8s.CustomResourceReference(
        CRD_GROUP, CRD_VERSION, POLICY_RESOURCE_PLURAL,
        policy_name, namespace="default",
    )
    k8s.create_custom_resource(ref, resource_data)
    cr = k8s.wait_resource_consumed_by_controller(ref)

    cr = k8s.get_resource(ref)
    assert cr is not None
    assert 'status' in cr
    assert 'ackResourceMetadata' in cr['status']
    assert 'arn' in cr['status']['ackResourceMetadata']
    policy_arn = cr['status']['ackResourceMetadata']['arn']

    policy.wait_until_exists(policy_arn)

    assert cr is not None
    assert k8s.get_resource_exists(ref)

    yield (ref, cr, policy_arn)

    _, deleted = k8s.delete_custom_resource(
        ref,
        period_length=DELETE_WAIT_AFTER_SECONDS,
    )
    assert deleted

    policy.wait_until_deleted(policy_arn)

@pytest.fixture
def adopt_policy(request):
    filename = ""
    resource_name = ""
    replacements = REPLACEMENT_VALUES.copy()

    marker = request.node.get_closest_marker("resource_data")
    assert marker is not None
    data = marker.args[0]
    assert 'adoption-policy' in data
    replacements["ADOPTION_POLICY"] = data['adoption-policy']
    assert 'filename' in data
    filename = data['filename']
    assert 'resource_name' in data
    resource_name = data['resource_name']

    resource_name = random_suffix_name(resource_name, 24)
    resource_arn = get_bootstrap_resources().AdoptedPolicy.arns[0]
    replacements["POLICY_ADOPTION_NAME"] = resource_name
    replacements["ADOPTION_FIELDS"] = f"{{\\\"arn\\\": \\\"{resource_arn}\\\"}}"
    replacements["POLICY_ADOPTION_NAME"] = resource_name

    resource_data = load_resource(
        filename,
        additional_replacements=replacements,
    )    

    ref = k8s.CustomResourceReference(
        CRD_GROUP, CRD_VERSION, POLICY_RESOURCE_PLURAL,
        resource_name, namespace="default",
    )
    k8s.create_custom_resource(ref, resource_data)

    time.sleep(CREATE_WAIT_AFTER_SECONDS)
    cr = k8s.wait_resource_consumed_by_controller(ref)
    assert cr is not None

    yield (ref, cr, resource_arn)



@service_marker
@pytest.mark.canary
class TestPolicy:
    def test_crud(self, simple_policy):
        ref, res, policy_arn = simple_policy

        time.sleep(CHECK_WAIT_AFTER_SECONDS)

        condition.assert_synced(ref)

        # check update code path for tags...
        latest_tags = policy.get_tags(policy_arn)
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

        condition.assert_synced(ref)

        latest_tags = policy.get_tags(policy_arn)
        after_update_expected_tags = [
            {
                "Key": "tag2",
                "Value": "val2",
            }
        ]
        assert tag.cleaned(latest_tags) == after_update_expected_tags
        new_tags = [
            {
                "key": "tag2",
                "value": "val3", # Update the value
            }
        ]
        updates = {
            "spec": {"tags": new_tags},
        }
        k8s.patch_custom_resource(ref, updates)
        time.sleep(MODIFY_WAIT_AFTER_SECONDS)

        condition.assert_synced(ref)

        latest_tags = policy.get_tags(policy_arn)
        after_update_expected_tags = [
            {
                "Key": "tag2",
                "Value": "val3",
            }
        ]
        assert tag.cleaned(latest_tags) == after_update_expected_tags

        # check update code path for policy document, which actually triggers
        # a call to CreatePolicyVersion...
        orig_policy_doc = {
            "Version":"2012-10-17",
            "Statement": [
                {
                    "Effect": "Allow",
                    "Action": "s3:ListAllMyBuckets",
                    "Resource": "arn:aws:s3:::*",
                },
                {
                    "Effect": "Allow",
                    "Action": ["s3:List*"],
                    "Resource": ["*"],
                },
            ],
        }

        cr = k8s.get_resource(ref)
        assert cr is not None
        assert "status" in cr
        assert "defaultVersionID" in cr["status"]
        assert cr["status"]["defaultVersionID"] == "v1"

        before_pv = policy.get_version(policy_arn, "v1")
        before_doc = before_pv["Document"]
        assert before_doc == orig_policy_doc

        new_policy_doc = {
            "Version":"2012-10-17",
            "Statement": [
                {
                    "Effect": "Allow",
                    "Action": "s3:ListAllMyBuckets",
                    "Resource": "arn:aws:s3:::*",
                },
            ],
        }
        updates = {
            "spec": {"policyDocument": json.dumps(new_policy_doc)},
        }
        k8s.patch_custom_resource(ref, updates)
        time.sleep(MODIFY_WAIT_AFTER_SECONDS)

        condition.assert_synced(ref)

        cr = k8s.get_resource(ref)
        assert cr is not None
        assert "status" in cr
        assert "defaultVersionID" in cr["status"]
        assert cr["status"]["defaultVersionID"] == "v2"

        after_pv = policy.get_version(policy_arn, "v2")
        after_doc = after_pv["Document"]
        assert after_doc == new_policy_doc

    @pytest.mark.resource_data({'adoption-policy': ADOPT_ADOPTION_POLICY, 'filename': 'policy_adopt', 'resource_name': 'adopt'})
    def test_policy_adopt_update(self, adopt_policy):
        ref, cr, policy_arn = adopt_policy

        k8s.wait_on_condition(ref, "ACK.ResourceSynced", "True", wait_periods=5)

        assert cr is not None
        assert 'status' in cr
        assert 'defaultVersionID' in cr['status']
        assert cr['status']['defaultVersionID'] == 'v1'

        new_policy_doc = {
            "Version":"2012-10-17",
            "Statement": [
                {
                    "Effect": "Allow",
                    "Action": "s3:ListAllMyBuckets",
                    "Resource": "*",
                },
            ],
        }

        updates = {
            "spec": {"policyDocument": json.dumps(new_policy_doc)},
        }
        k8s.patch_custom_resource(ref, updates)
        time.sleep(MODIFY_WAIT_AFTER_SECONDS)

        cr = k8s.get_resource(ref)
        assert cr is not None
        assert 'status' in cr
        assert 'defaultVersionID' in cr['status']
        assert cr['status']['defaultVersionID'] == 'v2'

        policy_doc = policy.get_version(policy_arn, "v2")["Document"]
        assert policy_doc == new_policy_doc

    @pytest.mark.resource_data({'adoption-policy': ADOPT_OR_CREATE_ADOPTION_POLICY, 'filename': 'policy_adopt_or_create', 'resource_name': 'adopt-or-create'})
    def test_policy_adopt_or_create(self, adopt_policy):
        ref, cr, policy_arn = adopt_policy

        k8s.wait_on_condition(ref, "ACK.ResourceSynced", "True", wait_periods=5)

        assert cr is not None
        assert 'status' in cr
        assert 'defaultVersionID' in cr['status']
        assert cr['status']['defaultVersionID'] == 'v1'
        assert 'ackResourceMetadata' in cr['status']
        assert 'arn' in cr['status']['ackResourceMetadata']
        assert cr['status']['ackResourceMetadata']['arn'] == policy_arn