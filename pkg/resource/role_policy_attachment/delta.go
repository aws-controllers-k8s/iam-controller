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

import (
	ackcompare "github.com/aws-controllers-k8s/runtime/pkg/compare"
	"k8s.io/apimachinery/pkg/api/equality"
)

func newResourceDelta(a *resource, b *resource) *ackcompare.Delta {
	delta := ackcompare.NewDelta()
	if (a == nil && b != nil) || (a != nil && b == nil) {
		delta.Add("", a, b)
		return delta
	}
	if ackcompare.HasNilDifference(a.ko.Spec.RoleName, b.ko.Spec.RoleName) {
		delta.Add("Spec.RoleName", a.ko.Spec.RoleName, b.ko.Spec.RoleName)
	} else if a.ko.Spec.RoleName != nil && b.ko.Spec.RoleName != nil && *a.ko.Spec.RoleName != *b.ko.Spec.RoleName {
		delta.Add("Spec.RoleName", a.ko.Spec.RoleName, b.ko.Spec.RoleName)
	}
	if ackcompare.HasNilDifference(a.ko.Spec.PolicyARN, b.ko.Spec.PolicyARN) {
		delta.Add("Spec.PolicyARN", a.ko.Spec.PolicyARN, b.ko.Spec.PolicyARN)
	} else if a.ko.Spec.PolicyARN != nil && b.ko.Spec.PolicyARN != nil && *a.ko.Spec.PolicyARN != *b.ko.Spec.PolicyARN {
		delta.Add("Spec.PolicyARN", a.ko.Spec.PolicyARN, b.ko.Spec.PolicyARN)
	}
	if !equality.Semantic.DeepEqual(a.ko.Spec.RoleRef, b.ko.Spec.RoleRef) {
		delta.Add("Spec.RoleRef", a.ko.Spec.RoleRef, b.ko.Spec.RoleRef)
	}
	if !equality.Semantic.DeepEqual(a.ko.Spec.PolicyRef, b.ko.Spec.PolicyRef) {
		delta.Add("Spec.PolicyRef", a.ko.Spec.PolicyRef, b.ko.Spec.PolicyRef)
	}
	return delta
}
