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

"""Utilities for working with User resources"""

import datetime
import time

import boto3
import pytest

DEFAULT_WAIT_UNTIL_EXISTS_TIMEOUT_SECONDS = 60*10
DEFAULT_WAIT_UNTIL_EXISTS_INTERVAL_SECONDS = 15
DEFAULT_WAIT_UNTIL_DELETED_TIMEOUT_SECONDS = 60*10
DEFAULT_WAIT_UNTIL_DELETED_INTERVAL_SECONDS = 15


def wait_until_exists(
        user_name: str,
        timeout_seconds: int = DEFAULT_WAIT_UNTIL_EXISTS_TIMEOUT_SECONDS,
        interval_seconds: int = DEFAULT_WAIT_UNTIL_EXISTS_INTERVAL_SECONDS,
    ) -> None:
    """Waits until a User with a supplied name is returned from IAM GetUser
    API.

    Usage:
        from e2e.user import wait_until_exists

        wait_until_exists(user_name)

    Raises:
        pytest.fail upon timeout
    """
    now = datetime.datetime.now()
    timeout = now + datetime.timedelta(seconds=timeout_seconds)

    while True:
        if datetime.datetime.now() >= timeout:
            pytest.fail(
                "Timed out waiting for User to exist "
                "in IAM API"
            )
        time.sleep(interval_seconds)

        latest = get(user_name)
        if latest is not None:
            break


def wait_until_deleted(
        user_name: str,
        timeout_seconds: int = DEFAULT_WAIT_UNTIL_DELETED_TIMEOUT_SECONDS,
        interval_seconds: int = DEFAULT_WAIT_UNTIL_DELETED_INTERVAL_SECONDS,
    ) -> None:
    """Waits until a User with a supplied ID is no longer returned from
    the IAM API.

    Usage:
        from e2e.user import wait_until_deleted

        wait_until_deleted(user_name)

    Raises:
        pytest.fail upon timeout
    """
    now = datetime.datetime.now()
    timeout = now + datetime.timedelta(seconds=timeout_seconds)

    while True:
        if datetime.datetime.now() >= timeout:
            pytest.fail(
                "Timed out waiting for User to be "
                "deleted in IAM API"
            )
        time.sleep(interval_seconds)

        latest = get(user_name)
        if latest is None:
            break


def get(user_name):
    """Returns a dict containing the User record from the IAM API.

    If no such User exists, returns None.
    """
    c = boto3.client('iam')
    try:
        resp = c.get_user(UserName=user_name)
        return resp['User']
    except c.exceptions.NoSuchEntityException:
        return None


def get_attached_policy_arns(user_name):
    """Returns a list containing the policy ARNs that have been attached to the
    supplied User.

    If no such User exists, returns None.
    """
    c = boto3.client('iam')
    try:
        resp = c.list_attached_user_policies(UserName=user_name)
        return [p['PolicyArn'] for p in resp['AttachedPolicies']]
    except c.exceptions.NoSuchEntityException:
        return None


def get_tags(user_name):
    """Returns a list containing the tags that have been associated to the
    supplied User.

    If no such User exists, returns None.
    """
    c = boto3.client('iam')
    try:
        resp = c.list_user_tags(UserName=user_name)
        return resp['Tags']
    except c.exceptions.NoSuchEntityException:
        return None
