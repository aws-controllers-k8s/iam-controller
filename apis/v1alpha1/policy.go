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

// PolicySpec defines the desired state of Policy.
//
// Contains information about a managed policy.
//
// This data type is used as a response element in the CreatePolicy, GetPolicy,
// and ListPolicies operations.
//
// For more information about managed policies, refer to Managed policies and
// inline policies (https://docs.aws.amazon.com/IAM/latest/UserGuide/policies-managed-vs-inline.html)
// in the IAM User Guide.
type PolicySpec struct {

	// A friendly description of the policy.
	//
	// Typically used to store information about the permissions defined in the
	// policy. For example, "Grants access to production DynamoDB tables."
	//
	// The policy description is immutable. After a value is assigned, it cannot
	// be changed.

	Description *string `json:"description,omitempty"`
	// The friendly name of the policy.
	//
	// IAM user, group, role, and policy names must be unique within the account.
	// Names are not distinguished by case. For example, you cannot create resources
	// named both "MyResource" and "myresource".

	// +kubebuilder:validation:Required

	Name *string `json:"name"`
	// The path for the policy.
	//
	// For more information about paths, see IAM identifiers (https://docs.aws.amazon.com/IAM/latest/UserGuide/Using_Identifiers.html)
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
	//
	// You cannot use an asterisk (*) in the path name.

	Path *string `json:"path,omitempty"`
	// The JSON policy document that you want to use as the content for the new
	// policy.
	//
	// You must provide policies in JSON format in IAM. However, for CloudFormation
	// templates formatted in YAML, you can provide the policy in JSON or YAML format.
	// CloudFormation always converts a YAML policy to JSON format before submitting
	// it to IAM.
	//
	// The maximum length of the policy document that you can pass in this operation,
	// including whitespace, is listed below. To view the maximum character counts
	// of a managed policy with no whitespaces, see IAM and STS character quotas
	// (https://docs.aws.amazon.com/IAM/latest/UserGuide/reference_iam-quotas.html#reference_iam-quotas-entity-length).
	//
	// To learn more about JSON policy grammar, see Grammar of the IAM JSON policy
	// language (https://docs.aws.amazon.com/IAM/latest/UserGuide/reference_policies_grammar.html)
	// in the IAM User Guide.
	//
	// The regex pattern (http://wikipedia.org/wiki/regex) used to validate this
	// parameter is a string of characters consisting of the following:
	//
	//    * Any printable ASCII character ranging from the space character (\u0020)
	//    through the end of the ASCII character range
	//
	//    * The printable characters in the Basic Latin and Latin-1 Supplement character
	//    set (through \u00FF)
	//
	//    * The special characters tab (\u0009), line feed (\u000A), and carriage
	//    return (\u000D)

	// +kubebuilder:validation:Required

	PolicyDocument *string `json:"policyDocument"`
	// A list of tags that you want to attach to the new IAM customer managed policy.
	// Each tag consists of a key name and an associated value. For more information
	// about tagging, see Tagging IAM resources (https://docs.aws.amazon.com/IAM/latest/UserGuide/id_tags.html)
	// in the IAM User Guide.
	//
	// If any one of the tags is invalid or if you exceed the allowed maximum number
	// of tags, then the entire request fails and the resource is not created.

	Tags []*Tag `json:"tags,omitempty"`
}

// PolicyStatus defines the observed state of Policy
type PolicyStatus struct {
	// All CRs managed by ACK have a common `Status.ACKResourceMetadata` member
	// that is used to contain resource sync state, account ownership,
	// constructed ARN for the resource
	// +kubebuilder:validation:Optional
	ACKResourceMetadata *ackv1alpha1.ResourceMetadata `json:"ackResourceMetadata"`
	// All CRs managed by ACK have a common `Status.Conditions` member that
	// contains a collection of `ackv1alpha1.Condition` objects that describe
	// the various terminal states of the CR and its backend AWS service API
	// resource
	// +kubebuilder:validation:Optional
	Conditions []*ackv1alpha1.Condition `json:"conditions"`
	// The number of entities (users, groups, and roles) that the policy is attached
	// to.
	// +kubebuilder:validation:Optional
	AttachmentCount *int64 `json:"attachmentCount,omitempty"`
	// The date and time, in ISO 8601 date-time format (http://www.iso.org/iso/iso8601),
	// when the policy was created.
	// +kubebuilder:validation:Optional
	CreateDate *metav1.Time `json:"createDate,omitempty"`
	// The identifier for the version of the policy that is set as the default version.
	// +kubebuilder:validation:Optional
	DefaultVersionID *string `json:"defaultVersionID,omitempty"`
	// Specifies whether the policy can be attached to an IAM user, group, or role.
	// +kubebuilder:validation:Optional
	IsAttachable *bool `json:"isAttachable,omitempty"`
	// The number of entities (users and roles) for which the policy is used to
	// set the permissions boundary.
	//
	// For more information about permissions boundaries, see Permissions boundaries
	// for IAM identities (https://docs.aws.amazon.com/IAM/latest/UserGuide/access_policies_boundaries.html)
	// in the IAM User Guide.
	// +kubebuilder:validation:Optional
	PermissionsBoundaryUsageCount *int64 `json:"permissionsBoundaryUsageCount,omitempty"`
	// The stable and unique string identifying the policy.
	//
	// For more information about IDs, see IAM identifiers (https://docs.aws.amazon.com/IAM/latest/UserGuide/Using_Identifiers.html)
	// in the IAM User Guide.
	// +kubebuilder:validation:Optional
	PolicyID *string `json:"policyID,omitempty"`
	// The date and time, in ISO 8601 date-time format (http://www.iso.org/iso/iso8601),
	// when the policy was last updated.
	//
	// When a policy has only one version, this field contains the date and time
	// when the policy was created. When a policy has more than one version, this
	// field contains the date and time when the most recent policy version was
	// created.
	// +kubebuilder:validation:Optional
	UpdateDate *metav1.Time `json:"updateDate,omitempty"`
}

// Policy is the Schema for the Policies API
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
type Policy struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              PolicySpec   `json:"spec,omitempty"`
	Status            PolicyStatus `json:"status,omitempty"`
}

// PolicyList contains a list of Policy
// +kubebuilder:object:root=true
type PolicyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Policy `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Policy{}, &PolicyList{})
}
