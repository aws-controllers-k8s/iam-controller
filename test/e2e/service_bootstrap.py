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

"""Bootstraps the resources required to run the IAM integration tests.
"""

import logging
import json

from acktest.bootstrapping import Resources, BootstrapFailureException
from acktest.bootstrapping.iam import UserPolicies, Role
from acktest.bootstrapping.cognito_identity import UserPool
from e2e import bootstrap_directory
from e2e.bootstrap_resources import BootstrapResources

def service_bootstrap() -> Resources:
    logging.getLogger().setLevel(logging.INFO)
    sample_policy = json.dumps({
        "Version": "2012-10-17",
        "Statement": [
            {
                "Effect": "Allow",
                "Action": [
                    "s3:ListBucket",
                ],
                "Resource": "*"
            }
        ]
    })
    resources = BootstrapResources(
        AdoptedPolicy=UserPolicies("adopted-policies", policy_documents=[sample_policy]),
        AdoptedRole=Role("adopted-role", "eks.amazonaws.com", managed_policies=["arn:aws:iam::aws:policy/AmazonSQSFullAccess", "arn:aws:iam::aws:policy/AmazonEC2FullAccess"]),
        OIDCProviderUserPool=UserPool(name_prefix="oidc-test-pool")
    )

    try:
        resources.bootstrap()
    except BootstrapFailureException as ex:
        exit(254)

    return resources

if __name__ == "__main__":
    config = service_bootstrap()
    # Write config to current directory by default
    config.serialize(bootstrap_directory)
