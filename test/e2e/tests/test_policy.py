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

from acktest.k8s import condition
from acktest.k8s import resource as k8s
from acktest.resources import random_suffix_name
from e2e import service_marker, CRD_GROUP, CRD_VERSION, load_resource
from e2e.common.types import POLICY_RESOURCE_PLURAL
from e2e.replacement_values import REPLACEMENT_VALUES
from e2e import policy
from e2e import tag

DELETE_WAIT_AFTER_SECONDS = 10
CHECK_WAIT_AFTER_SECONDS = 10
# NOTE(jaypipes): I've seen Tagris take nearly 20 seconds to return updated tag
# information on a resource.
MODIFY_WAIT_AFTER_SECONDS = 20


@service_marker
@pytest.mark.canary
class TestPolicy:
    def test_crud(self):
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

        time.sleep(CHECK_WAIT_AFTER_SECONDS)

        cr = k8s.get_resource(ref)
        assert cr is not None
        assert 'status' in cr
        assert 'ackResourceMetadata' in cr['status']
        assert 'arn' in cr['status']['ackResourceMetadata']
        policy_arn = cr['status']['ackResourceMetadata']['arn']

        condition.assert_synced(ref)

        policy.wait_until_exists(policy_arn)

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

        k8s.delete_custom_resource(ref)

        time.sleep(DELETE_WAIT_AFTER_SECONDS)

        policy.wait_until_deleted(policy_arn)
