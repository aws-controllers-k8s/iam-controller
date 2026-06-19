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

package v1alpha1

import (
	ackv1alpha1 "github.com/aws-controllers-k8s/runtime/apis/core/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// RolePolicyAttachmentSpec defines the desired state of RolePolicyAttachment.
type RolePolicyAttachmentSpec struct {
	PolicyARN *string                                  `json:"policyARN,omitempty"`
	PolicyRef *ackv1alpha1.AWSResourceReferenceWrapper `json:"policyRef,omitempty"`
	RoleName  *string                                  `json:"roleName,omitempty"`
	RoleRef   *ackv1alpha1.AWSResourceReferenceWrapper `json:"roleRef,omitempty"`
}

// RolePolicyAttachmentStatus defines the observed state of RolePolicyAttachment.
type RolePolicyAttachmentStatus struct {
	// +kubebuilder:validation:Optional
	ACKResourceMetadata *ackv1alpha1.ResourceMetadata `json:"ackResourceMetadata"`
	// +kubebuilder:validation:Optional
	Conditions []*ackv1alpha1.Condition `json:"conditions"`
	// +kubebuilder:validation:Optional
	Attached *bool `json:"attached,omitempty"`
}

// RolePolicyAttachment is the Schema for the RolePolicyAttachments API.
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
type RolePolicyAttachment struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              RolePolicyAttachmentSpec   `json:"spec,omitempty"`
	Status            RolePolicyAttachmentStatus `json:"status,omitempty"`
}

// RolePolicyAttachmentList contains a list of RolePolicyAttachment.
// +kubebuilder:object:root=true
type RolePolicyAttachmentList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []RolePolicyAttachment `json:"items"`
}

func init() {
	SchemeBuilder.Register(&RolePolicyAttachment{}, &RolePolicyAttachmentList{})
}
