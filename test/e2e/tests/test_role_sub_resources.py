# Copyright Amazon.com Inc. or its affiliates. All Rights Reserved.
#
# Licensed under the Apache License, Version 2.0 (the "License"). You may
# not use this file except in compliance with the License. A copy of the
# License is located at
#
#     http://aws.amazon.com/apache2.0/
#
# or in the "license" file accompanying this file. This file is distributed
# on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either
# express or implied. See the License for the specific language governing
# permissions and limitations under the License.

"""Individual CRUD tests for each Role sub-resource field managed by
sub-resource managers: Policies, InlinePolicies, Tags,
PermissionsBoundary, AssumeRolePolicyDocument."""

import json
import time

import pytest

from acktest.k8s import condition
from acktest.k8s import resource as k8s
from acktest.resources import random_suffix_name
from e2e import service_marker, CRD_GROUP, CRD_VERSION, load_resource
from e2e.common.types import ROLE_RESOURCE_PLURAL
from e2e.replacement_values import REPLACEMENT_VALUES
from e2e import role
from e2e import tag

WAIT_SECONDS = 8


@pytest.fixture(scope="module")
def base_role():
    """Creates a minimal Role for sub-resource tests."""
    role_name = random_suffix_name("sub-res-role", 24)
    replacements = REPLACEMENT_VALUES.copy()
    replacements['ROLE_NAME'] = role_name
    replacements['ROLE_DESCRIPTION'] = "sub-resource test role"
    replacements['MAX_SESSION_DURATION'] = "3600"

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

    yield ref, cr, role_name

    k8s.delete_custom_resource(ref, period_length=WAIT_SECONDS)
    role.wait_until_deleted(role_name)


@service_marker
class TestPoliciesSubResource:
    """CRUD tests for the Policies (managed policies) sub-resource."""

    def test_add_policies(self, base_role):
        ref, _, role_name = base_role
        policy_arns = [
            "arn:aws:iam::aws:policy/AmazonS3ReadOnlyAccess",
            "arn:aws:iam::aws:policy/AmazonSQSReadOnlyAccess",
        ]
        k8s.patch_custom_resource(ref, {"spec": {"policies": policy_arns}})
        time.sleep(WAIT_SECONDS)
        condition.assert_synced(ref)

        latest = role.get_attached_policy_arns(role_name)
        assert set(latest) == set(policy_arns)

        cr = k8s.get_resource(ref)
        assert set(cr['spec']['policies']) == set(policy_arns)

    def test_replace_policies(self, base_role):
        ref, _, role_name = base_role
        # Swap SQS for SNS
        policy_arns = [
            "arn:aws:iam::aws:policy/AmazonS3ReadOnlyAccess",
            "arn:aws:iam::aws:policy/AmazonSNSReadOnlyAccess",
        ]
        k8s.patch_custom_resource(ref, {"spec": {"policies": policy_arns}})
        time.sleep(WAIT_SECONDS)
        condition.assert_synced(ref)

        latest = role.get_attached_policy_arns(role_name)
        assert set(latest) == set(policy_arns)

    def test_remove_all_policies(self, base_role):
        ref, _, role_name = base_role
        k8s.patch_custom_resource(ref, {"spec": {"policies": []}})
        time.sleep(WAIT_SECONDS)
        condition.assert_synced(ref)

        latest = role.get_attached_policy_arns(role_name)
        assert len(latest) == 0


@service_marker
class TestInlinePoliciesSubResource:
    """CRUD tests for the InlinePolicies sub-resource."""

    EC2_DOC = '{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Action":["ec2:Get*"],"Resource":["*"]}]}'
    S3_DOC = '{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Action":["s3:Get*"],"Resource":["*"]}]}'
    S3_OBJ_DOC = '{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Action":["s3:GetObject"],"Resource":["*"]}]}'

    def test_add_inline_policies(self, base_role):
        ref, _, role_name = base_role
        updates = {
            "spec": {
                "inlinePolicies": {
                    "ec2get": self.EC2_DOC,
                    "s3get": self.S3_DOC,
                },
            },
        }
        k8s.patch_custom_resource(ref, updates)
        time.sleep(WAIT_SECONDS)
        condition.assert_synced(ref)

        latest = role.get_inline_policies(role_name)
        assert len(latest) == 2
        assert "ec2get" in latest
        assert "s3get" in latest

    def test_update_inline_policy_document(self, base_role):
        ref, _, role_name = base_role
        updates = {
            "spec": {
                "inlinePolicies": {
                    "ec2get": self.EC2_DOC,
                    "s3get": self.S3_OBJ_DOC,
                },
            },
        }
        k8s.patch_custom_resource(ref, updates)
        time.sleep(WAIT_SECONDS)
        condition.assert_synced(ref)

        latest = role.get_inline_policies(role_name)
        nospace = lambda s: "".join(c for c in s if not c.isspace())
        assert nospace(latest['s3get']) == nospace(self.S3_OBJ_DOC)

    def test_remove_all_inline_policies(self, base_role):
        ref, _, role_name = base_role
        k8s.patch_custom_resource(ref, {"spec": {"inlinePolicies": None}})
        time.sleep(WAIT_SECONDS)
        condition.assert_synced(ref)

        latest = role.get_inline_policies(role_name)
        assert len(latest) == 0


@service_marker
class TestTagsSubResource:
    """CRUD tests for the Tags sub-resource."""

    def test_add_tags(self, base_role):
        ref, _, role_name = base_role
        new_tags = [{"key": "env", "value": "test"}]
        k8s.patch_custom_resource(ref, {"spec": {"tags": new_tags}})
        time.sleep(WAIT_SECONDS)
        condition.assert_synced(ref)

        latest = role.get_tags(role_name)
        assert {"Key": "env", "Value": "test"} in tag.cleaned(latest)

    def test_update_tags(self, base_role):
        ref, _, role_name = base_role
        updated_tags = [
            {"key": "env", "value": "prod"},
            {"key": "team", "value": "platform"},
        ]
        k8s.patch_custom_resource(ref, {"spec": {"tags": updated_tags}})
        time.sleep(WAIT_SECONDS)
        condition.assert_synced(ref)

        latest = tag.cleaned(role.get_tags(role_name))
        assert {"Key": "env", "Value": "prod"} in latest
        assert {"Key": "team", "Value": "platform"} in latest

    def test_remove_tags(self, base_role):
        ref, _, role_name = base_role
        k8s.patch_custom_resource(ref, {"spec": {"tags": []}})
        time.sleep(WAIT_SECONDS)
        condition.assert_synced(ref)

        latest = role.get_tags(role_name)
        assert len(tag.cleaned(latest)) == 0


@service_marker
class TestPermissionsBoundarySubResource:
    """CRUD tests for the PermissionsBoundary sub-resource."""

    BOUNDARY_ARN = "arn:aws:iam::aws:policy/AmazonS3ReadOnlyAccess"

    def test_set_permissions_boundary(self, base_role):
        ref, _, role_name = base_role
        k8s.patch_custom_resource(
            ref, {"spec": {"permissionsBoundary": self.BOUNDARY_ARN}},
        )
        time.sleep(WAIT_SECONDS)
        condition.assert_synced(ref)

        latest = role.get(role_name)
        assert latest["PermissionsBoundary"]["PermissionsBoundaryArn"] == self.BOUNDARY_ARN

    def test_remove_permissions_boundary(self, base_role):
        ref, _, role_name = base_role
        k8s.patch_custom_resource(
            ref, {"spec": {"permissionsBoundary": None}},
        )
        time.sleep(WAIT_SECONDS)
        condition.assert_synced(ref)

        latest = role.get(role_name)
        assert latest.get("PermissionsBoundary") is None


@service_marker
class TestAssumeRolePolicyDocumentSubResource:
    """CRU tests for the AssumeRolePolicyDocument sub-resource.
    (Cannot be deleted — only updated.)"""

    ALLOW_DOC = json.dumps({
        "Version": "2012-10-17",
        "Statement": [{
            "Effect": "Allow",
            "Principal": {"Service": ["ec2.amazonaws.com"]},
            "Action": ["sts:AssumeRole"],
        }],
    })
    DENY_DOC = json.dumps({
        "Version": "2012-10-17",
        "Statement": [{
            "Effect": "Deny",
            "Principal": {"Service": ["ec2.amazonaws.com"]},
            "Action": ["sts:AssumeRole"],
        }],
    })

    def test_read_assume_role_policy(self, base_role):
        ref, _, role_name = base_role
        time.sleep(WAIT_SECONDS)
        condition.assert_synced(ref)

        cr = k8s.get_resource(ref)
        assert 'assumeRolePolicyDocument' in cr['spec']
        doc = json.loads(cr['spec']['assumeRolePolicyDocument'])
        assert doc['Statement'][0]['Effect'] == 'Allow'

    def test_update_assume_role_policy(self, base_role):
        ref, _, role_name = base_role
        k8s.patch_custom_resource(
            ref, {"spec": {"assumeRolePolicyDocument": self.DENY_DOC}},
        )
        time.sleep(WAIT_SECONDS)
        condition.assert_synced(ref)

        cr = k8s.get_resource(ref)
        doc = json.loads(cr['spec']['assumeRolePolicyDocument'])
        assert doc['Statement'][0]['Effect'] == 'Deny'

        latest = role.get_assume_role_policy(role_name)
        assert latest['Statement'][0]['Effect'] == 'Deny'
