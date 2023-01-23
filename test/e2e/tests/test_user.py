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

"""Integration tests for the IAM User resource"""

import logging
import time

import pytest

from acktest.k8s import condition
from acktest.k8s import resource as k8s
from acktest.resources import random_suffix_name
from e2e import service_marker, CRD_GROUP, CRD_VERSION, load_resource
from e2e.common.types import USER_RESOURCE_PLURAL
from e2e.replacement_values import REPLACEMENT_VALUES
from e2e import user
from e2e import tag

DELETE_WAIT_AFTER_SECONDS = 10
CHECK_STATUS_WAIT_SECONDS = 10
MODIFY_WAIT_AFTER_SECONDS = 10


@pytest.fixture(scope="module")
def simple_user():
    user_name = random_suffix_name("my-simple-user", 24)

    replacements = REPLACEMENT_VALUES.copy()
    replacements['USER_NAME'] = user_name

    resource_data = load_resource(
        "user_simple",
        additional_replacements=replacements,
    )

    ref = k8s.CustomResourceReference(
        CRD_GROUP, CRD_VERSION, USER_RESOURCE_PLURAL,
        user_name, namespace="default",
    )
    k8s.create_custom_resource(ref, resource_data)
    cr = k8s.wait_resource_consumed_by_controller(ref)

    user.wait_until_exists(user_name)

    assert cr is not None
    assert k8s.get_resource_exists(ref)

    yield (ref, cr)

    _, deleted = k8s.delete_custom_resource(
        ref,
        period_length=DELETE_WAIT_AFTER_SECONDS,
    )
    assert deleted

    user.wait_until_deleted(user_name)


@service_marker
@pytest.mark.canary
class TestUser:
    def test_crud(self, simple_user):
        ref, res = simple_user
        user_name = ref.name

        time.sleep(CHECK_STATUS_WAIT_SECONDS)

        condition.assert_synced(ref)

        latest = user.get(user_name)
        assert latest is not None
        assert latest['UserName'] == user_name

        latest_policy_arns = user.get_attached_policy_arns(user_name)
        assert len(latest_policy_arns) == 0

        # Test the code paths that synchronize the attached policies for a user
        policy_arns = [
            "arn:aws:iam::aws:policy/AmazonS3ReadOnlyAccess",
        ]
        permissionsBoundary = 'arn:aws:iam::aws:policy/AmazonS3ReadOnlyAccess'
        new_path = "/engineering/"
        updates = {
            "spec": {
                "policies": policy_arns,
                "path": new_path,
                "permissionsBoundary": permissionsBoundary,
            },
        }
        k8s.patch_custom_resource(ref, updates)
        time.sleep(MODIFY_WAIT_AFTER_SECONDS)

        latest_policy_arns = user.get_attached_policy_arns(user_name)
        assert latest_policy_arns == policy_arns

        latest_user = user.get(user_name)
        assert latest_user["Path"] == new_path
        assert latest_user["PermissionsBoundary"]["PermissionsBoundaryArn"] == permissionsBoundary

        # Same update code path check for tags...
        latest_tags = user.get_tags(user_name)
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
        latest_tags = user.get_tags(user_name)
        assert tag.cleaned(latest_tags) == after_update_expected_tags
