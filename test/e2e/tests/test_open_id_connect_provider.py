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

"""Integration tests for the IAM OIDCProvider resource"""

import logging
import time

import pytest

from acktest.k8s import condition
from acktest.k8s import resource as k8s
from acktest.resources import random_suffix_name
from e2e import service_marker, CRD_GROUP, CRD_VERSION, load_resource
from e2e.replacement_values import REPLACEMENT_VALUES
from e2e import open_id_connect_provider
from e2e import tag

RESOURCE_PLURAL = "openidconnectproviders"

DELETE_WAIT_AFTER_SECONDS = 5
CHECK_STATUS_WAIT_SECONDS = 5
MODIFY_WAIT_AFTER_SECONDS = 10


@pytest.fixture
def oidc_provider():
    # required:
    #   url
    #   list of client IDs
    #   list of server cert thumbprints (see https://docs.aws.amazon.com/IAM/latest/UserGuide/id_roles_providers_create_oidc_verify-thumbprint.html)
    # optional:
    #   a list of tags
    oidc_provider_name = random_suffix_name("oidc-provider-ack-test", 24)

    replacements = REPLACEMENT_VALUES.copy()
    replacements["OPEN_ID_CONNECT_PROVIDER_NAME"] = oidc_provider_name
    replacements["URL"] = "https://host.domain.net"
    replacements["CLIENT_ID"] = "phippy"
    # thumbprints must be exactly 40 characters
    replacements["THUMBPRINT"] = "0123456789012345678901234567890123456789"
    replacements["TAG_KEY"] = "tag1"
    replacements["TAG_VALUE"] = "val1"

    resource_data = load_resource(
        "open_id_connect_provider_simple",
        additional_replacements=replacements,
    )

    logging.info(f"**** Pytest fixture creating: {resource_data}")
    ref = k8s.CustomResourceReference(
        CRD_GROUP,
        CRD_VERSION,
        RESOURCE_PLURAL,
        oidc_provider_name,
        namespace="default",
    )
    k8s.create_custom_resource(ref, resource_data)
    cr = k8s.wait_resource_consumed_by_controller(ref)

    logging.info(f"**** Pytest fixture created: {cr['spec']}")
    yield (ref, cr)

    # Delete the OIDC provider when tests complete
    try:
        _, deleted = k8s.delete_custom_resource(ref, 3, 10)
        assert deleted
    except:
        pass


@service_marker
@pytest.mark.canary
class TestOpenIdConnectProvider:
    def test_crud(self, oidc_provider):
        (ref, cr) = oidc_provider

        # get the ARN
        logging.debug(f"\n\n**** OIDCProvider create")
        cr = k8s.get_resource(ref)
        assert cr is not None
        assert "status" in cr
        assert "ackResourceMetadata" in cr["status"]
        logging.debug(f"ackResourceMetadata: {cr['status']['ackResourceMetadata']}")
        assert "arn" in cr["status"]["ackResourceMetadata"]
        oidc_provider_arn = cr["status"]["ackResourceMetadata"]["arn"]

        logging.info(f"\n\n**** OIDCProvider ARN created: {oidc_provider_arn}")
        logging.info(f"\n\n**** OIDCProvider Spec created: {cr['spec']}")

        open_id_connect_provider.wait_until_exists(oidc_provider_arn)

        time.sleep(CHECK_STATUS_WAIT_SECONDS)

        condition.assert_synced(ref)

        logging.debug(f"\n\n**** OIDCProvider create validation")
        latest = open_id_connect_provider.get(oidc_provider_arn)
        logging.info(f"\n**** OIDCProvider created: {latest}")

        assert latest is not None
        assert len(latest["ThumbprintList"]) == 1
        assert latest["Url"] == "host.domain.net"

        # perform an update to some part of the OIDCProvider
        logging.info(f"\n\n**** OIDCProvider update")

        new_thumbprints = [
            "9876543210987654321098765432109876543210"
        ]  # thumbprints must be 40 characters
        updates = {
            "spec": {
                "thumbprintList": new_thumbprints,
                "tags": [{"key": "tag2", "value": "val2"}],
            },
        }
        k8s.patch_custom_resource(ref, updates)
        time.sleep(MODIFY_WAIT_AFTER_SECONDS)

        logging.info(f"\n\n**** OIDCProvider update validation")
        latest = open_id_connect_provider.get(oidc_provider_arn)
        assert latest is not None
        logging.info(f"\n\n**** OIDCProvider updated: {latest}")
        assert len(latest["ThumbprintList"]) == 1
        assert latest["ThumbprintList"][0] == new_thumbprints[0]

        after_update_expected_tags = [{"Key": "key2", "Value": "val2"}]
        logging.info(f"\n\n**** OIDCProvider validate tags update")
        latest_tags = open_id_connect_provider.get_tags(oidc_provider_arn)
        assert tag.cleaned(latest_tags) == after_update_expected_tags
