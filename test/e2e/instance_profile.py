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

"""Utilities for working with Instance Profile resources"""

import boto3


def get_instance_profile(instance_profile_name):
    c = boto3.client('iam')
    try:
        resp = c.get_instance_profile(InstanceProfileName=instance_profile_name)
        return resp
    except c.exceptions.NoSuchEntityException:
        return None


def get_tags(instance_profile_name):
    c = boto3.client('iam')
    try:
        resp = c.list_instance_profile_tags(InstanceProfileName=instance_profile_name)
        return resp['Tags']
    except c.exceptions.NoSuchEntityException:
        return None
