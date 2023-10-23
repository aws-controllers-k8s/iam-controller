# Copyright Amazon.com Inc. or its affiliates. All Rights Reserved.
#
# Licensed under the Apache License, Version 2.0 (the "License"). You may
# not use this file except in compliance with the License. A copy of the
# License is located at
#
# 	 http://aws.amazon.com/apache2.0/
#
# or in the "license" file accompanying this file. This file is distributed
# on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either
# express or implied. See the License for the specific language governing
# permissions and limitations under the License.

"""Integration tests for the IAM InstanceProfile resource"""

import pytest
import time

from acktest.k8s import resource as k8s
from acktest.resources import random_suffix_name
from e2e import service_marker, CRD_GROUP, CRD_VERSION, load_resource
from e2e import role
from e2e import instance_profile
from e2e.replacement_values import REPLACEMENT_VALUES

CHECK_STATUS_WAIT_SECONDS = 5
DELETE_WAIT_AFTER_SECONDS = 5
MODIFY_WAIT_AFTER_SECONDS = 10


@pytest.fixture(scope="function")
def simple_instance_profile():
    instance_profile_name = random_suffix_name("test-profile", 24)

    replacements = REPLACEMENT_VALUES.copy()
    replacements["INSTANCE_PROFILE_NAME"] = instance_profile_name
    replacements["TAG_KEY"] = "tag1"
    replacements["TAG_VALUE"] = "val1"

    resource_data = load_resource(
        "instance_profile_simple",
        additional_replacements=replacements,
    )

    ref = k8s.CustomResourceReference(
        CRD_GROUP,
        CRD_VERSION,
        "instanceprofiles",
        instance_profile_name,
        namespace="default",
    )
    k8s.create_custom_resource(ref, resource_data)
    cr = k8s.wait_resource_consumed_by_controller(ref)

    yield (ref, cr)

    # Delete the instance profile when tests complete
    try:
        _, deleted = k8s.delete_custom_resource(ref, 3, 10)
        assert deleted
    except:
        pass


@pytest.fixture(scope="function")
def simple_role():
    role_name = random_suffix_name("my-simple-role", 24)
    role_desc = "a simple role"

    replacements = REPLACEMENT_VALUES.copy()
    replacements['ROLE_NAME'] = role_name
    replacements['ROLE_DESCRIPTION'] = role_desc
    replacements['MAX_SESSION_DURATION'] = "3600"

    resource_data = load_resource(
        "role_simple",
        additional_replacements=replacements,
    )

    ref = k8s.CustomResourceReference(
        CRD_GROUP, CRD_VERSION, "roles",
        role_name, namespace="default",
    )
    k8s.create_custom_resource(ref, resource_data)
    cr = k8s.wait_resource_consumed_by_controller(ref)

    role.wait_until_exists(role_name)

    yield (ref, cr)

    _, deleted = k8s.delete_custom_resource(
        ref,
        period_length=DELETE_WAIT_AFTER_SECONDS,
    )
    assert deleted

    role.wait_until_deleted(role_name)


@service_marker
@pytest.mark.canary
class TestInstanceProfile:
    def test_crud(self, simple_role, simple_instance_profile):
        # Create a role to be attached to an instance profile
        (ref, cr) = simple_role
        assert cr is not None
        assert k8s.get_resource_exists(ref)
        role_name = cr["metadata"]["name"]

        # Create an instance profile with no role attached
        (ref, cr) = simple_instance_profile
        assert cr is not None
        assert k8s.get_resource_exists(ref)
        instance_profile_name = cr["metadata"]["name"]

        # Ensure that the instance profile was registered by AWS
        assert "status" in cr
        assert "instanceProfileID" in cr["status"]
        assert "createDate" in cr["status"]

        # Ensure that late initialized specs have been initialized
        assert "path" in cr["spec"]

        # Verify no role is assigned to the instance profile
        profile = instance_profile.get_instance_profile(instance_profile_name)
        assert len(profile["InstanceProfile"]["Roles"]) == 0

        # Update the instance profile with an IAM role, our code should
        # sync changes and attach the role to the instance profile
        updates = {
            "spec": {
                "role": role_name,
                "tags": [{"key": "tag2", "value": "val2"}],
            },
        }
        k8s.patch_custom_resource(ref, updates)
        time.sleep(MODIFY_WAIT_AFTER_SECONDS)

        # Verify the newly attached role is assigned to the instance profile
        profile = instance_profile.get_instance_profile(instance_profile_name)
        assert len(profile["InstanceProfile"]["Roles"]) > 0

        # Verify tags have been updated
        after_update_expected_tags = {"Key": "tag2", "Value": "val2"}
        updated_instance_profile = instance_profile.get_instance_profile(instance_profile_name)
        latest_tags = updated_instance_profile["InstanceProfile"]["Tags"]
        assert after_update_expected_tags in latest_tags
