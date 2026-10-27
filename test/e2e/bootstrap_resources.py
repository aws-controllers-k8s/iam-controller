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

"""Declares the structure of the bootstrapped resources and provides a loader
for them.
"""

from dataclasses import dataclass
from acktest.bootstrapping import Resources
from acktest.bootstrapping.iam import UserPolicies, Role
from acktest.bootstrapping.cognito_identity import UserPool
from e2e import bootstrap_directory

@dataclass
class BootstrapResources(Resources):
    AdoptedPolicy: UserPolicies
    # AdoptedPolicyForUpdate is a dedicated policy for tests that mutate the
    # adopted policy's version (e.g. test_policy_adopt_update). Keeping it
    # separate from AdoptedPolicy prevents a cross-test race under pytest-xdist:
    # a mutating test bumping the shared policy to v2 would otherwise make a
    # concurrent test that asserts the pristine v1 state flake.
    AdoptedPolicyForUpdate: UserPolicies
    AdoptedRole: Role
    OIDCProviderUserPool: UserPool
_bootstrap_resources = None

def get_bootstrap_resources(bootstrap_file_name: str = "bootstrap.pkl") -> BootstrapResources:
    global _bootstrap_resources
    if _bootstrap_resources is None:
        _bootstrap_resources = BootstrapResources.deserialize(bootstrap_directory, bootstrap_file_name=bootstrap_file_name)
    return _bootstrap_resources
