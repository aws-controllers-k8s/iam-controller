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

"""Integration tests for resource description fields
The reason we are dedicating a test file for this is because the description/lateinitialize
bug is the most common bug we have seen in the ACK project. This test file is dedicated
to testing the description field for policy and role resources.

See bugs:
- 
"""

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
from e2e import role

DELETE_WAIT_SECONDS = 10
CREATE_WAIT_SECONDS = 10
MODIFY_WAIT_SECONDS = 10


@pytest.fixture(scope="module")
def policy_with_no_description():
    policy_name = random_suffix_name("my-simple-policy", 24)

    replacements = REPLACEMENT_VALUES.copy()
    replacements['POLICY_NAME'] = policy_name

    resource_data = load_resource(
        "policy_no_description",
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

    yield (ref, cr)

    _, deleted = k8s.delete_custom_resource(
        ref,
        period_length=DELETE_WAIT_SECONDS,
    )
    assert deleted

    policy.wait_until_deleted(policy_arn)

@pytest.fixture(scope="module")
def role_with_no_description():
    role_name = random_suffix_name("my-simple-role", 24)

    replacements = REPLACEMENT_VALUES.copy()
    replacements['ROLE_NAME'] = role_name

    resource_data = load_resource(
        "role_no_description",
        additional_replacements=replacements,
    )

    ref = k8s.CustomResourceReference(
        CRD_GROUP, CRD_VERSION, "roles",
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
        period_length=DELETE_WAIT_SECONDS,
    )
    assert deleted

    role.wait_until_deleted(role_name)

@service_marker
@pytest.mark.canary
class TestRole:
    def test_role_empty_description(self, role_with_no_description):
        ref, res = role_with_no_description
        role_name = ref.name

        time.sleep(CREATE_WAIT_SECONDS)
        condition.assert_ready(ref)
        condition.assert_type_status(
            ref,
            cond_type_match=condition.CONDITION_TYPE_LATE_INITIALIZED,
            cond_status_match=True,
        )

        updates = {
            "spec": {
                "description": "non empty description",
            },
        }
        k8s.patch_custom_resource(ref, updates)
        time.sleep(MODIFY_WAIT_SECONDS)
        condition.assert_ready(ref)

        latest = role.get(role_name)
        assert latest is not None
        assert latest["Description"] == "non empty description"

    def test_policy_empty_description(self, policy_with_no_description):
        ref, res = policy_with_no_description

        time.sleep(CREATE_WAIT_SECONDS)
        condition.assert_ready(ref)
        condition.assert_type_status(
            ref,
            cond_type_match=condition.CONDITION_TYPE_LATE_INITIALIZED,
            cond_status_match=True,
        )

