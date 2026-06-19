// Copyright Amazon.com Inc. or its affiliates. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License"). You may
// not use this file except in compliance with the License. A copy of the
// License is located at
//
//     http://aws.amazon.com/apache2.0/
//
// or in the "license" file accompanying this file. This file is distributed
// on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either
// express or implied. See the License for the specific language governing
// permissions and limitations under the License.

package role_policy_attachment

import ackv1alpha1 "github.com/aws-controllers-k8s/runtime/apis/core/v1alpha1"

type resourceIdentifiers struct {
	meta *ackv1alpha1.ResourceMetadata
}

func (ri *resourceIdentifiers) ARN() *ackv1alpha1.AWSResourceName {
	if ri.meta != nil {
		return ri.meta.ARN
	}
	return nil
}

func (ri *resourceIdentifiers) OwnerAccountID() *ackv1alpha1.AWSAccountID {
	if ri.meta != nil {
		return ri.meta.OwnerAccountID
	}
	return nil
}

func (ri *resourceIdentifiers) Region() *ackv1alpha1.AWSRegion {
	if ri.meta != nil {
		return ri.meta.Region
	}
	return nil
}

func (ri *resourceIdentifiers) Partition() *ackv1alpha1.AWSPartition {
	if ri.meta != nil {
		return ri.meta.Partition
	}
	return nil
}
