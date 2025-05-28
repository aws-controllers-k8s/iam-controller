# Copyright Amazon.com Inc. or its affiliates. All Rights Reserved.
#
# Licensed under the Apache License, Version 2.0 (the "License"). You may
# not use this file except in compliance with the License. A copy of the
# License is located at
#
#    http://aws.amazon.com/apache2.0/
#
# or in the "license" file accompanying this file. This file is distributed
# on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either
# express or implied. See the License for the specific language governing
# permissions and limitations under the License.

"""Integration tests for the IAM Service-Linked Role resource"""

import time
import pytest
from acktest.k8s import condition
from acktest.k8s import resource as k8s
from acktest.resources import random_suffix_name
from e2e import service_marker, CRD_GROUP, CRD_VERSION, load_resource
from e2e.common.types import SERVICE_LINKED_ROLE_RESOURCE_PLURAL
from e2e.replacement_values import REPLACEMENT_VALUES
from e2e import role

DELETE_WAIT_AFTER_SECONDS = 10
CHECK_STATUS_WAIT_SECONDS = 10
ELASTICBEANSTALK_SERVICE_LINKED_ROLE_NAME="AWSServiceRoleForElasticBeanstalk"

@pytest.fixture(scope="module")
def service_linked_role():
    role_name = random_suffix_name("my-simple-role", 24)

    replacements = REPLACEMENT_VALUES.copy()
    replacements["ROLE_NAME"] = role_name
    replacements['ROLE_DESCRIPTION'] = "a service-linked role"
    replacements['AWS_SERVICE_NAME'] = "elasticbeanstalk.amazonaws.com"

    resource_data = load_resource(
        "service_linked_role",
        additional_replacements=replacements,
    )

    service_linked_role=ELASTICBEANSTALK_SERVICE_LINKED_ROLE_NAME

    ref = k8s.CustomResourceReference(
        CRD_GROUP, CRD_VERSION, SERVICE_LINKED_ROLE_RESOURCE_PLURAL,
        role_name, namespace="default",
    )
    k8s.create_custom_resource(ref, resource_data)
    cr = k8s.wait_resource_consumed_by_controller(ref)

    role.wait_until_exists(service_linked_role)

    assert cr is not None
    assert k8s.get_resource_exists(ref)

    yield (ref, cr)

    _, deleted = k8s.delete_custom_resource(
        ref,
        period_length=DELETE_WAIT_AFTER_SECONDS,
    )
    assert deleted

    role.wait_until_deleted(service_linked_role)

@service_marker
@pytest.mark.canary
class TestServiceLinkedRole:
    def test_crud(self, service_linked_role):
        ref, res = service_linked_role
        role_name = ELASTICBEANSTALK_SERVICE_LINKED_ROLE_NAME

        time.sleep(CHECK_STATUS_WAIT_SECONDS)

        condition.assert_synced(ref)

        # Validate the service-linked role exists
        latest = role.get(role_name)

        assert latest is not None
        assert latest['Description'] == "a service-linked role"
        assert latest['Path'] == "/aws-service-role/elasticbeanstalk.amazonaws.com/"

        # Update the service-linked role (if applicable)
        updates = {
            "spec": {
                "description": "an updated service-linked role",
            },
        }
        k8s.patch_custom_resource(ref, updates)
        time.sleep(CHECK_STATUS_WAIT_SECONDS)

        condition.assert_synced(ref)

        latest = role.get(role_name)
        assert latest is not None
        assert latest["Description"] == "an updated service-linked role"