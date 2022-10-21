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

"""Utilities for working with OpenIdConnectProvider resources"""

import datetime
import time

import boto3
import pytest

DEFAULT_WAIT_UNTIL_EXISTS_TIMEOUT_SECONDS = 60*10
DEFAULT_WAIT_UNTIL_EXISTS_INTERVAL_SECONDS = 15
DEFAULT_WAIT_UNTIL_DELETED_TIMEOUT_SECONDS = 60*10
DEFAULT_WAIT_UNTIL_DELETED_INTERVAL_SECONDS = 15


def wait_until_exists(
        oidc_arn: str,
        timeout_seconds: int = DEFAULT_WAIT_UNTIL_EXISTS_TIMEOUT_SECONDS,
        interval_seconds: int = DEFAULT_WAIT_UNTIL_EXISTS_INTERVAL_SECONDS,
    ) -> None:
    """Waits until an OpenIdConnectProvider with a supplied ARN is returned from the IAM API.

    Usage:
        from e2e.open_id_connect_provider import wait_until_exists

        wait_until_exists(oidc_provider_arn)

    Raises:
        pytest.fail upon timeout
    """
    now = datetime.datetime.now()
    timeout = now + datetime.timedelta(seconds=timeout_seconds)

    while True:
        if datetime.datetime.now() >= timeout:
            pytest.fail(
                "Timed out waiting for OpenIdConnectProvider to exist "
                "in IAM API"
            )
        time.sleep(interval_seconds)

        latest = get(oidc_arn)
        if latest is not None:
            break


def wait_until_deleted(
        oidc_arn: str,
        timeout_seconds: int = DEFAULT_WAIT_UNTIL_DELETED_TIMEOUT_SECONDS,
        interval_seconds: int = DEFAULT_WAIT_UNTIL_DELETED_INTERVAL_SECONDS,
    ) -> None:
    """Waits until an OpenIdConnectProvider with a supplied ARN is no longer returned from the IAM API.

    Usage:
        from e2e.open_id_connect_provider import wait_until_deleted

        wait_until_deleted(oidc_provider_arn)

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

        latest = get(oidc_arn)
        if latest is None:
            break


def get(oidc_provider_arn):
    """Returns a dict containing the OpenIDConnectProvider record from the IAM API.

    If no such OpenIDConnectProvider exists, returns None.
    """
    c = boto3.client('iam')
    try:
        resp = c.get_open_id_connect_provider(OpenIDConnectProviderArn=oidc_provider_arn)
        return resp
    except c.exceptions.NoSuchEntityException:
        return None


def get_tags(oidc_provider_arn):
    """Returns a list containing the tags that have been associated to the
    supplied OpenIDConnectProvider.

    If no such Role exists, returns None.
    """
    c = boto3.client('iam')
    try:
        resp = c.list_open_id_connect_provider_tags(OpenIDConnectProviderArn=oidc_provider_arn)
        return resp['Tags']
    except c.exceptions.NoSuchEntityException:
        return None
