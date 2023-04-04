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

"""Utilities for working with Role resources"""

import datetime
import json
import time

import boto3
import pytest

DEFAULT_WAIT_UNTIL_EXISTS_TIMEOUT_SECONDS = 60*10
DEFAULT_WAIT_UNTIL_EXISTS_INTERVAL_SECONDS = 15
DEFAULT_WAIT_UNTIL_DELETED_TIMEOUT_SECONDS = 60*10
DEFAULT_WAIT_UNTIL_DELETED_INTERVAL_SECONDS = 15


def wait_until_exists(
        role_name: str,
        timeout_seconds: int = DEFAULT_WAIT_UNTIL_EXISTS_TIMEOUT_SECONDS,
        interval_seconds: int = DEFAULT_WAIT_UNTIL_EXISTS_INTERVAL_SECONDS,
    ) -> None:
    """Waits until a Role with a supplied name is returned from IAM GetRole
    API.

    Usage:
        from e2e.role import wait_until_exists

        wait_until_exists(role_name)

    Raises:
        pytest.fail upon timeout
    """
    now = datetime.datetime.now()
    timeout = now + datetime.timedelta(seconds=timeout_seconds)

    while True:
        if datetime.datetime.now() >= timeout:
            pytest.fail(
                "Timed out waiting for Role to exist "
                "in IAM API"
            )
        time.sleep(interval_seconds)

        latest = get(role_name)
        if latest is not None:
            break


def wait_until_deleted(
        role_name: str,
        timeout_seconds: int = DEFAULT_WAIT_UNTIL_DELETED_TIMEOUT_SECONDS,
        interval_seconds: int = DEFAULT_WAIT_UNTIL_DELETED_INTERVAL_SECONDS,
    ) -> None:
    """Waits until a Role with a supplied ID is no longer returned from
    the IAM API.

    Usage:
        from e2e.role import wait_until_deleted

        wait_until_deleted(role_name)

    Raises:
        pytest.fail upon timeout
    """
    now = datetime.datetime.now()
    timeout = now + datetime.timedelta(seconds=timeout_seconds)

    while True:
        if datetime.datetime.now() >= timeout:
            pytest.fail(
                "Timed out waiting for Role to be "
                "deleted in IAM API"
            )
        time.sleep(interval_seconds)

        latest = get(role_name)
        if latest is None:
            break


def get(role_name):
    """Returns a dict containing the Role record from the IAM API.

    If no such Role exists, returns None.
    """
    c = boto3.client('iam')
    try:
        resp = c.get_role(RoleName=role_name)
        return resp['Role']
    except c.exceptions.NoSuchEntityException:
        return None


def get_attached_policy_arns(role_name):
    """Returns a list containing the policy ARNs that have been attached to the
    supplied Role.

    If no such Role exists, returns None.
    """
    c = boto3.client('iam')
    try:
        resp = c.list_attached_role_policies(RoleName=role_name)
        return [p['PolicyArn'] for p in resp['AttachedPolicies']]
    except c.exceptions.NoSuchEntityException:
        return None


def get_tags(role_name):
    """Returns a list containing the tags that have been associated to the
    supplied Role.

    If no such Role exists, returns None.
    """
    c = boto3.client('iam')
    try:
        resp = c.list_role_tags(RoleName=role_name)
        return resp['Tags']
    except c.exceptions.NoSuchEntityException:
        return None


def get_inline_policies(role_name):
    """Returns a dict containing the policy names for inline policies that have
    been attached to the supplied Role along with the policy document values.

    If no such Role exists, returns None.
    """
    c = boto3.client('iam')
    try:
        resp = c.list_role_policies(RoleName=role_name)
        policies = {}
        for pol_name in resp['PolicyNames']:
            pol_resp = c.get_role_policy(
                RoleName=role_name, PolicyName=pol_name,
            )
            policies[pol_name] = json.dumps(pol_resp['PolicyDocument'])
        return policies
    except c.exceptions.NoSuchEntityException:
        return None

def get_assume_role_policy(role_name):
    """Returns a dict representing the assume role policy document for the supplied Role.

    If no such Role exists, returns None.
    """
    c = boto3.client('iam')
    try:
        resp = get(role_name)
        return resp['AssumeRolePolicyDocument']
    except c.exceptions.NoSuchEntityException:
        return None

