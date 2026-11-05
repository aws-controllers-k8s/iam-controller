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

var (
	// AnnotationPrefix is the prefix for all annotations specifically for
	// the IAM service.
	AnnotationPrefix = GroupVersion.Group + "/"
	// AnnotationReadyAfterCreate is an annotation whose value is a Go
	// duration (e.g. "15s"). When set on a Role, the controller keeps
	// ACK.ResourceSynced=False until that much time has passed since the
	// role's CreateDate, to cover IAM eventual consistency: a newly created
	// role can be returned by GetRole several seconds before STS accepts
	// AssumeRole* calls for it. A role whose CreateDate is already older than
	// the window (for example an adopted role) is not delayed.
	AnnotationReadyAfterCreate = AnnotationPrefix + "ready-after-create"
)
