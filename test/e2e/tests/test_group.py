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
