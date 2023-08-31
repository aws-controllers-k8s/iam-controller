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

// Code generated by ack-generate. DO NOT EDIT.

package v1alpha1

import (
	ackv1alpha1 "github.com/aws-controllers-k8s/runtime/apis/core/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// InstanceProfileSpec defines the desired state of InstanceProfile.
//
// Contains information about an instance profile.
//
// This data type is used as a response element in the following operations:
//
//   - CreateInstanceProfile
//
//   - GetInstanceProfile
//
//   - ListInstanceProfiles
//
//   - ListInstanceProfilesForRole
type InstanceProfileSpec struct {

	// The name of the instance profile to create.
	//
	// This parameter allows (through its regex pattern (http://wikipedia.org/wiki/regex))
	// a string of characters consisting of upper and lowercase alphanumeric characters
	// with no spaces. You can also include any of the following characters: _+=,.@-
	// +kubebuilder:validation:Required
	Name *string `json:"name"`
	// The path to the instance profile. For more information about paths, see IAM
	// Identifiers (https://docs.aws.amazon.com/IAM/latest/UserGuide/Using_Identifiers.html)
	// in the IAM User Guide.
	//
	// This parameter is optional. If it is not included, it defaults to a slash
	// (/).
	//
	// This parameter allows (through its regex pattern (http://wikipedia.org/wiki/regex))
	// a string of characters consisting of either a forward slash (/) by itself
	// or a string that must begin and end with forward slashes. In addition, it
	// can contain any ASCII character from the ! (\u0021) through the DEL character
	// (\u007F), including most punctuation characters, digits, and upper and lowercased
	// letters.
	Path    *string                                  `json:"path,omitempty"`
	Role    *string                                  `json:"role,omitempty"`
	RoleRef *ackv1alpha1.AWSResourceReferenceWrapper `json:"roleRef,omitempty"`
	// A list of tags that you want to attach to the newly created IAM instance
	// profile. Each tag consists of a key name and an associated value. For more
	// information about tagging, see Tagging IAM resources (https://docs.aws.amazon.com/IAM/latest/UserGuide/id_tags.html)
	// in the IAM User Guide.
	//
	// If any one of the tags is invalid or if you exceed the allowed maximum number
	// of tags, then the entire request fails and the resource is not created.
	Tags []*Tag `json:"tags,omitempty"`
}

// InstanceProfileStatus defines the observed state of InstanceProfile
type InstanceProfileStatus struct {
	// All CRs managed by ACK have a common `Status.ACKResourceMetadata` member
	// that is used to contain resource sync state, account ownership,
	// constructed ARN for the resource
	// +kubebuilder:validation:Optional
	ACKResourceMetadata *ackv1alpha1.ResourceMetadata `json:"ackResourceMetadata"`
	// All CRS managed by ACK have a common `Status.Conditions` member that
	// contains a collection of `ackv1alpha1.Condition` objects that describe
	// the various terminal states of the CR and its backend AWS service API
	// resource
	// +kubebuilder:validation:Optional
	Conditions []*ackv1alpha1.Condition `json:"conditions"`
	// The date when the instance profile was created.
	// +kubebuilder:validation:Optional
	CreateDate *metav1.Time `json:"createDate,omitempty"`
	// The stable and unique string identifying the instance profile. For more information
	// about IDs, see IAM identifiers (https://docs.aws.amazon.com/IAM/latest/UserGuide/Using_Identifiers.html)
	// in the IAM User Guide.
	// +kubebuilder:validation:Optional
	InstanceProfileID *string `json:"instanceProfileID,omitempty"`
}

// InstanceProfile is the Schema for the InstanceProfiles API
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
type InstanceProfile struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              InstanceProfileSpec   `json:"spec,omitempty"`
	Status            InstanceProfileStatus `json:"status,omitempty"`
}

// InstanceProfileList contains a list of InstanceProfile
// +kubebuilder:object:root=true
type InstanceProfileList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []InstanceProfile `json:"items"`
}

func init() {
	SchemeBuilder.Register(&InstanceProfile{}, &InstanceProfileList{})
}