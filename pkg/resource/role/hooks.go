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

package role

import (
	"encoding/json"
	"net/url"

	ackcompare "github.com/aws-controllers-k8s/runtime/pkg/compare"
	awsiampolicy "github.com/micahhausler/aws-iam-policy/policy"

	commonutil "github.com/aws-controllers-k8s/iam-controller/pkg/util"
)

// customPreCompare contains logic that help compare two iam Roles. This
// function is injected in newResourceDelta function.
func customPreCompare(
	delta *ackcompare.Delta,
	a *resource,
	b *resource,
) {
	compareTags(delta, a, b)
	compareAssumeRolePolicyDocument(delta, a, b)
}

// compareTags is a custom comparison function for comparing lists of Tag
// structs where the order of the structs in the list is not important.
func compareTags(
	delta *ackcompare.Delta,
	a *resource,
	b *resource,
) {
	if len(a.ko.Spec.Tags) != len(b.ko.Spec.Tags) {
		delta.Add("Spec.Tags", a.ko.Spec.Tags, b.ko.Spec.Tags)
	} else if len(a.ko.Spec.Tags) > 0 {
		if !commonutil.EqualTags(a.ko.Spec.Tags, b.ko.Spec.Tags) {
			delta.Add("Spec.Tags", a.ko.Spec.Tags, b.ko.Spec.Tags)
		}
	}
}

// compareAssumeRolePolicyDocument is a custom comparison function for
// assumeRolePolicyDocuments. The reason why we need a custom function for
// this fields is the API logic that trims all the trailing while spaces
// string of provided documents.
func compareAssumeRolePolicyDocument(
	delta *ackcompare.Delta,
	a *resource,
	b *resource,
) {
	if a.ko.Spec.AssumeRolePolicyDocument == nil && b.ko.Spec.AssumeRolePolicyDocument == nil {
		return
	}
	if a.ko.Spec.AssumeRolePolicyDocument == nil || b.ko.Spec.AssumeRolePolicyDocument == nil {
		delta.Add("Spec.AssumeRolePolicyDocument", a.ko.Spec.AssumeRolePolicyDocument, b.ko.Spec.AssumeRolePolicyDocument)
		return
	}
	// Normalize both documents through the aws-iam-policy library by
	// unmarshaling and re-marshaling. This handles AWS normalizations like
	// single-element arrays being collapsed to strings (e.g.
	// "Service": ["ec2.amazonaws.com"] → "Service": "ec2.amazonaws.com").
	var policyA, policyB awsiampolicy.Policy
	if err := json.Unmarshal([]byte(*a.ko.Spec.AssumeRolePolicyDocument), &policyA); err != nil {
		delta.Add("Spec.AssumeRolePolicyDocument", a.ko.Spec.AssumeRolePolicyDocument, b.ko.Spec.AssumeRolePolicyDocument)
		return
	}
	if err := json.Unmarshal([]byte(*b.ko.Spec.AssumeRolePolicyDocument), &policyB); err != nil {
		delta.Add("Spec.AssumeRolePolicyDocument", a.ko.Spec.AssumeRolePolicyDocument, b.ko.Spec.AssumeRolePolicyDocument)
		return
	}
	normalizedA, errA := json.Marshal(policyA)
	normalizedB, errB := json.Marshal(policyB)
	if errA != nil || errB != nil {
		delta.Add("Spec.AssumeRolePolicyDocument", a.ko.Spec.AssumeRolePolicyDocument, b.ko.Spec.AssumeRolePolicyDocument)
		return
	}
	if string(normalizedA) != string(normalizedB) {
		delta.Add("Spec.AssumeRolePolicyDocument", a.ko.Spec.AssumeRolePolicyDocument, b.ko.Spec.AssumeRolePolicyDocument)
	}
}

func decodeDocument(encoded string) (string, error) {
	return url.QueryUnescape(encoded)
}
