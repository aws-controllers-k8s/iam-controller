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
from e2e.bootstrap_resources import get_bootstrap_resources
from e2e.replacement_values import REPLACEMENT_VALUES
from e2e import role
from e2e import tag

DELETE_WAIT_AFTER_SECONDS = 10
CHECK_STATUS_WAIT_SECONDS = 10
MODIFY_WAIT_AFTER_SECONDS = 10
MAX_SESS_DURATION = 3600 # Note: minimum of 3600 seconds...
ROLE_DESC = "a simple role"


@pytest.fixture(scope="module")
def simple_role():
    role_name = random_suffix_name("my-simple-role", 24)

    replacements = REPLACEMENT_VALUES.copy()
    replacements['ROLE_NAME'] = role_name
    replacements['ROLE_DESCRIPTION'] = ROLE_DESC
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

@pytest.fixture(scope="module")
def adopt_role():
    resource_name = get_bootstrap_resources().AdoptedRole.name
    replacements = REPLACEMENT_VALUES.copy()
    replacements['ROLE_ADOPTION_NAME'] = resource_name
    replacements['ADOPTION_POLICY'] = "adopt"
    replacements['ADOPTION_FIELDS'] = f"{{\\\"name\\\": \\\"{resource_name}\\\"}}"

    resource_data = load_resource(
        "role_adoption",
        additional_replacements=replacements,
    )

    ref = k8s.CustomResourceReference(
        CRD_GROUP, CRD_VERSION, ROLE_RESOURCE_PLURAL,
        resource_name, namespace="default",
    )
    k8s.create_custom_resource(ref, resource_data)

    time.sleep(CHECK_STATUS_WAIT_SECONDS)
    cr = k8s.wait_resource_consumed_by_controller(ref)
    assert cr is not None

    yield (ref, cr)


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
        # Check that the Description field has not been removed.
        # See: https://github.com/aws-controllers-k8s/community/issues/1772
        assert 'description' in cr['spec']
        assert cr['spec']['description'] == ROLE_DESC

        condition.assert_synced(ref)

        latest = role.get(role_name)
        assert latest is not None
        assert latest['MaxSessionDuration'] == MAX_SESS_DURATION

        new_max_sess_duration = MAX_SESS_DURATION + 100

        # We're now going to modify the MaxSessionDuration field of the Role,
        # wait some time and verify that the IAM server-side resource
        # shows the new value of the field.
        updates = {
            "spec": {
                "maxSessionDuration": new_max_sess_duration,
                "description": "a simple role with a new max session duration",
            },
        }
        k8s.patch_custom_resource(ref, updates)
        time.sleep(MODIFY_WAIT_AFTER_SECONDS)

        condition.assert_synced(ref)

        latest = role.get(role_name)
        assert latest is not None
        assert latest['MaxSessionDuration'] == new_max_sess_duration
        assert latest["Description"] == "a simple role with a new max session duration"

        # Sub-resource fields (Policies, PermissionsBoundary, Tags,
        # InlinePolicies, AssumeRolePolicyDocument) are tested in
        # test_role_sub_resources.py

        condition.assert_synced(ref)

    def test_role_adopt(self, adopt_role):
        ref, cr = adopt_role

        condition.assert_synced(ref)

        assert cr is not None
        assert 'status' in cr
        assert 'spec' in cr
