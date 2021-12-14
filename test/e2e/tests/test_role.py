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
import time

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


@service_marker
@pytest.mark.canary
class TestRole:
    def test_crud(self):
        role_name = "my-simple-role"
        role_desc = "a simple role"
        max_sess_duration = 3600 # Note: minimum of 3600 seconds...

        replacements = REPLACEMENT_VALUES.copy()
        replacements['ROLE_NAME'] = role_name
        replacements['ROLE_DESCRIPTION'] = role_desc
        replacements['MAX_SESSION_DURATION'] = str(max_sess_duration)

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

        time.sleep(CHECK_STATUS_WAIT_SECONDS)

        # Before we update the Role CR below, let's check to see that the
        # MaxSessionDuration field in the CR is still what we set in the
        # original Create call.
        cr = k8s.get_resource(ref)
        assert cr is not None
        assert 'spec' in cr
        assert 'maxSessionDuration' in cr['spec']
        assert cr['spec']['maxSessionDuration'] == max_sess_duration

        condition.assert_synced(ref)

        latest = role.get(role_name)
        assert latest is not None
        assert latest['MaxSessionDuration'] == max_sess_duration

        new_max_sess_duration = max_sess_duration + 100

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
        updates = {
            "spec": {"policies": policy_arns},
        }
        k8s.patch_custom_resource(ref, updates)
        time.sleep(MODIFY_WAIT_AFTER_SECONDS)

        latest_policy_arns = role.get_attached_policy_arns(role_name)
        assert latest_policy_arns == policy_arns

        k8s.delete_custom_resource(ref)

        time.sleep(DELETE_WAIT_AFTER_SECONDS)

        role.wait_until_deleted(role_name)
