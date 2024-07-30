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

"""Integration tests for resource references"""

import json
import time

import pytest

import logging 

from acktest.k8s import condition
from acktest.k8s import resource as k8s
from acktest.resources import random_suffix_name
from e2e import service_marker, CRD_GROUP, CRD_VERSION, load_resource
from e2e.common.types import POLICY_RESOURCE_PLURAL, ROLE_RESOURCE_PLURAL
from e2e.replacement_values import REPLACEMENT_VALUES
from e2e import role
from e2e import policy
from e2e import tag

DELETE_ROLE_TIMEOUT_SECONDS = 10
# Little longer to delete the policy since it's referred-to from the role...
DELETE_POLICY_TIMEOUT_SECONDS = 30
CHECK_WAIT_AFTER_REF_RESOLVE_SECONDS = 10


@pytest.fixture(scope="module")
def referred_policy_name():
    return random_suffix_name("referred-policy", 24)

@pytest.fixture(scope="module")
def referred_policy_namspace():
    return random_suffix_name("policy-namespace", 24)


@pytest.fixture(scope="function")
def referring_role(request, referred_policy_name, referred_policy_namspace):
    role_name = random_suffix_name("referring-role", 24)

    marker = request.node.get_closest_marker("resource_data")
    filename = "role_referring"
    replacements = REPLACEMENT_VALUES.copy()

    if marker is not None:
        data = marker.args[0]
        if 'withNamespace' in data and data['withNamespace']:
            filename = "role_referring_namespace"
            replacements['POLICY_NAMESPACE'] = referred_policy_namspace

    replacements['ROLE_NAME'] = role_name
    replacements['POLICY_NAME'] = referred_policy_name

    resource_data = load_resource(
        filename,
        additional_replacements=replacements,
    )

    ref = k8s.CustomResourceReference(
        CRD_GROUP, CRD_VERSION, ROLE_RESOURCE_PLURAL,
        role_name, namespace="default",
    )
    k8s.create_custom_resource(ref, resource_data)
    cr = k8s.wait_resource_consumed_by_controller(ref)

    assert cr is not None
    assert k8s.get_resource_exists(ref)

    # NOTE(jaypipes): We specifically do NOT wait for the Role to exist in
    # the IAM API here because we will create the referred-to Policy and
    # wait for the reference to be resolved

    yield (ref, cr, role_name)

    if k8s.get_resource_exists(ref):
        # If all goes properly, we should not hit this because the test cleans
        # up the child resource before exiting...
        _, deleted = k8s.delete_custom_resource(
            ref,
            period_length=DELETE_ROLE_TIMEOUT_SECONDS,
        )
        assert deleted

        role.wait_until_deleted(role_name)


@pytest.fixture(scope="function")
def referred_policy(request, referred_policy_name, referred_policy_namspace):
    policy_desc = "a referred-to policy"

    marker = request.node.get_closest_marker("resource_data")
    filename = "policy_simple"
    namespace = "default"

    replacements = REPLACEMENT_VALUES.copy()

    if marker is not None:
        data = marker.args[0]
        if 'withNamespace' in data and data['withNamespace']:
            namespace = referred_policy_namspace
            k8s.create_k8s_namespace(
                namespace
            )
            time.sleep(CHECK_WAIT_AFTER_REF_RESOLVE_SECONDS)
            filename = "policy_simple_namespace"
            replacements['POLICY_NAMESPACE'] = namespace
            
    replacements['POLICY_NAME'] = referred_policy_name
    replacements['POLICY_DESCRIPTION'] = policy_desc

    resource_data = load_resource(
        filename,
        additional_replacements=replacements,
    )

    ref = k8s.CustomResourceReference(
        CRD_GROUP, CRD_VERSION, POLICY_RESOURCE_PLURAL,
        referred_policy_name, namespace=namespace,
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
        period_length=DELETE_POLICY_TIMEOUT_SECONDS,
    )
    assert deleted

    policy.wait_until_deleted(policy_arn)


@service_marker
@pytest.mark.canary
class TestReferences:
    @pytest.mark.resource_data({'withNamespace': False})
    def test_role_policy_references(self, referring_role, referred_policy):

        # create the resources in order that initially the reference resolution
        # fails and then when the referenced resource gets created, then all
        # resolutions eventually pass and resources get synced.
        role_ref, role_cr, role_name = referring_role

        time.sleep(1)

        policy_ref, policy_cr, policy_arn = referred_policy

        time.sleep(CHECK_WAIT_AFTER_REF_RESOLVE_SECONDS)

        condition.assert_synced(policy_ref)
        condition.assert_synced(role_ref)

        role.wait_until_exists(role_name)

        # NOTE(jaypipes): We need to manually delete the Role first because
        # pytest fixtures will try to clean up the Policy fixture *first*
        # (because it was initialized after Role) but if we try to delete the
        # Role before the Policy, the cascading delete protection of resource
        # references will mean the Role won't be deleted.
        _, deleted = k8s.delete_custom_resource(
            role_ref,
            period_length=DELETE_ROLE_TIMEOUT_SECONDS,
        )
        assert deleted

        role.wait_until_deleted(role_name)
    
    @pytest.mark.resource_data({'withNamespace': True})
    def test_role_policy_namespace_references(self, referring_role, referred_policy):

        # create the resources in order that initially the reference resolution
        # fails and then when the referenced resource gets created, then all
        # resolutions eventually pass and resources get synced.
        time.sleep(CHECK_WAIT_AFTER_REF_RESOLVE_SECONDS)

        role_ref, role_cr, role_name = referring_role

        time.sleep(1)

        policy_ref, policy_cr, policy_arn = referred_policy

        time.sleep(CHECK_WAIT_AFTER_REF_RESOLVE_SECONDS)

        condition.assert_synced(policy_ref)
        condition.assert_synced(role_ref)

        role.wait_until_exists(role_name)

        # NOTE(jaypipes): We need to manually delete the Role first because
        # pytest fixtures will try to clean up the Policy fixture *first*
        # (because it was initialized after Role) but if we try to delete the
        # Role before the Policy, the cascading delete protection of resource
        # references will mean the Role won't be deleted.
        _, deleted = k8s.delete_custom_resource(
            role_ref,
            period_length=DELETE_ROLE_TIMEOUT_SECONDS,
        )
        assert deleted

        role.wait_until_deleted(role_name)
    
