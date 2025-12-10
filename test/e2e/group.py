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

"""Utilities for working with Group resources"""

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
        group_name: str,
        timeout_seconds: int = DEFAULT_WAIT_UNTIL_EXISTS_TIMEOUT_SECONDS,
        interval_seconds: int = DEFAULT_WAIT_UNTIL_EXISTS_INTERVAL_SECONDS,
    ) -> None:
    """Waits until a Group with a supplied name is returned from IAM GetGroup
    API.

    Usage:
        from e2e.group import wait_until_exists

        wait_until_exists(group_name)

    Raises:
        pytest.fail upon timeout
    """
    now = datetime.datetime.now()
    timeout = now + datetime.timedelta(seconds=timeout_seconds)

    while True:
        if datetime.datetime.now() >= timeout:
            pytest.fail(
                "Timed out waiting for Group to exist "
                "in IAM API"
            )
        time.sleep(interval_seconds)

        latest = get(group_name)
        if latest is not None:
            break


def wait_until_deleted(
        group_name: str,
        timeout_seconds: int = DEFAULT_WAIT_UNTIL_DELETED_TIMEOUT_SECONDS,
        interval_seconds: int = DEFAULT_WAIT_UNTIL_DELETED_INTERVAL_SECONDS,
    ) -> None:
    """Waits until a Group with a supplied ID is no longer returned from
    the IAM API.

    Usage:
        from e2e.group import wait_until_deleted

        wait_until_deleted(group_name)

    Raises:
        pytest.fail upon timeout
    """
    now = datetime.datetime.now()
    timeout = now + datetime.timedelta(seconds=timeout_seconds)

    while True:
        if datetime.datetime.now() >= timeout:
            pytest.fail(
                "Timed out waiting for Group to be "
                "deleted in IAM API"
            )
        time.sleep(interval_seconds)

        latest = get(group_name)
        if latest is None:
            break


def get(group_name):
    """Returns a dict containing the Group record from the IAM API.

    If no such Group exists, returns None.
    """
    c = boto3.client('iam')
    try:
        resp = c.get_group(GroupName=group_name)
        return resp['Group']
    except c.exceptions.NoSuchEntityException:
        return None


def get_attached_policy_arns(group_name):
    """Returns a list containing the policy ARNs that have been attached to the
    supplied Group.

    If no such Group exists, returns None.
    """
    c = boto3.client('iam')
    try:
        resp = c.list_attached_group_policies(GroupName=group_name)
        return [p['PolicyArn'] for p in resp['AttachedPolicies']]
    except c.exceptions.NoSuchEntityException:
        return None


def get_inline_policies(group_name):
    """Returns a dict containing the policy names for inline policies that have
    been attached to the supplied Group along with the policy document values.

    If no such Group exists, returns None.
    """
    c = boto3.client('iam')
    try:
        resp = c.list_group_policies(GroupName=group_name)
        policies = {}
        for pol_name in resp['PolicyNames']:
            pol_resp = c.get_group_policy(
                GroupName=group_name, PolicyName=pol_name,
            )
            policies[pol_name] = json.dumps(pol_resp['PolicyDocument'])
        return policies
    except c.exceptions.NoSuchEntityException:
        return None


def get_users(group_name):
    """Returns a list containing the user names that are members of the
    supplied Group.

    If no such Group exists, returns None.
    """
    c = boto3.client('iam')
    try:
        users = []
        paginator = c.get_paginator('get_group')
        for page in paginator.paginate(GroupName=group_name):
            for user in page['Users']:
                users.append(user['UserName'])
        return users
    except c.exceptions.NoSuchEntityException:
        return None
