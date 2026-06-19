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
	ackv1alpha1 "github.com/aws-controllers-k8s/runtime/apis/core/v1alpha1"
	ackcompare "github.com/aws-controllers-k8s/runtime/pkg/compare"
	acktypes "github.com/aws-controllers-k8s/runtime/pkg/types"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	rtclient "sigs.k8s.io/controller-runtime/pkg/client"
	k8sctrlutil "sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	svcapitypes "github.com/aws-controllers-k8s/iam-controller/apis/v1alpha1"
)

const (
	FinalizerString = "finalizers.iam.services.k8s.aws/RolePolicyAttachment"
)

var (
	GroupVersionResource = svcapitypes.GroupVersion.WithResource("rolepolicyattachments")
	GroupKind            = metav1.GroupKind{Group: "iam.services.k8s.aws", Kind: "RolePolicyAttachment"}
)

type resourceDescriptor struct{}

func (d *resourceDescriptor) GroupVersionKind() schema.GroupVersionKind {
	return svcapitypes.GroupVersion.WithKind(GroupKind.Kind)
}

func (d *resourceDescriptor) EmptyRuntimeObject() rtclient.Object {
	return &svcapitypes.RolePolicyAttachment{}
}

func (d *resourceDescriptor) ResourceFromRuntimeObject(obj rtclient.Object) acktypes.AWSResource {
	return &resource{ko: obj.(*svcapitypes.RolePolicyAttachment)}
}

func (d *resourceDescriptor) Delta(a, b acktypes.AWSResource) *ackcompare.Delta {
	return newResourceDelta(a.(*resource), b.(*resource))
}

func (d *resourceDescriptor) IsManaged(res acktypes.AWSResource) bool {
	obj := res.RuntimeObject()
	if obj == nil {
		panic("nil RuntimeMetaObject in AWSResource")
	}
	return containsFinalizer(obj, FinalizerString)
}

func containsFinalizer(obj rtclient.Object, finalizer string) bool {
	for _, existing := range obj.GetFinalizers() {
		if existing == finalizer {
			return true
		}
	}
	return false
}

func (d *resourceDescriptor) MarkManaged(res acktypes.AWSResource) {
	obj := res.RuntimeObject()
	if obj == nil {
		panic("nil RuntimeMetaObject in AWSResource")
	}
	k8sctrlutil.AddFinalizer(obj, FinalizerString)
}

func (d *resourceDescriptor) MarkUnmanaged(res acktypes.AWSResource) {
	obj := res.RuntimeObject()
	if obj == nil {
		panic("nil RuntimeMetaObject in AWSResource")
	}
	k8sctrlutil.RemoveFinalizer(obj, FinalizerString)
}

func (d *resourceDescriptor) MarkAdopted(res acktypes.AWSResource) {
	obj := res.RuntimeObject()
	if obj == nil {
		panic("nil RuntimeObject in AWSResource")
	}
	annotations := obj.GetAnnotations()
	if annotations == nil {
		annotations = map[string]string{}
	}
	annotations[ackv1alpha1.AnnotationAdopted] = "true"
	obj.SetAnnotations(annotations)
}
