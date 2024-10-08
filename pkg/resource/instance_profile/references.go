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

package instance_profile

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	ackv1alpha1 "github.com/aws-controllers-k8s/runtime/apis/core/v1alpha1"
	ackerr "github.com/aws-controllers-k8s/runtime/pkg/errors"
	acktypes "github.com/aws-controllers-k8s/runtime/pkg/types"

	svcapitypes "github.com/aws-controllers-k8s/iam-controller/apis/v1alpha1"
)

// ClearResolvedReferences removes any reference values that were made
// concrete in the spec. It returns a copy of the input AWSResource which
// contains the original *Ref values, but none of their respective concrete
// values.
func (rm *resourceManager) ClearResolvedReferences(res acktypes.AWSResource) acktypes.AWSResource {
	ko := rm.concreteResource(res).ko.DeepCopy()

	if ko.Spec.RoleRef != nil {
		ko.Spec.Role = nil
	}

	return &resource{ko}
}

// ResolveReferences finds if there are any Reference field(s) present
// inside AWSResource passed in the parameter and attempts to resolve those
// reference field(s) into their respective target field(s). It returns a
// copy of the input AWSResource with resolved reference(s), a boolean which
// is set to true if the resource contains any references (regardless of if
// they are resolved successfully) and an error if the passed AWSResource's
// reference field(s) could not be resolved.
func (rm *resourceManager) ResolveReferences(
	ctx context.Context,
	apiReader client.Reader,
	res acktypes.AWSResource,
) (acktypes.AWSResource, bool, error) {
	ko := rm.concreteResource(res).ko

	resourceHasReferences := false
	err := validateReferenceFields(ko)
	if fieldHasReferences, err := rm.resolveReferenceForRole(ctx, apiReader, ko); err != nil {
		return &resource{ko}, (resourceHasReferences || fieldHasReferences), err
	} else {
		resourceHasReferences = resourceHasReferences || fieldHasReferences
	}

	return &resource{ko}, resourceHasReferences, err
}

// validateReferenceFields validates the reference field and corresponding
// identifier field.
func validateReferenceFields(ko *svcapitypes.InstanceProfile) error {

	if ko.Spec.RoleRef != nil && ko.Spec.Role != nil {
		return ackerr.ResourceReferenceAndIDNotSupportedFor("Role", "RoleRef")
	}
	return nil
}

// resolveReferenceForRole reads the resource referenced
// from RoleRef field and sets the Role
// from referenced resource. Returns a boolean indicating whether a reference
// contains references, or an error
func (rm *resourceManager) resolveReferenceForRole(
	ctx context.Context,
	apiReader client.Reader,
	ko *svcapitypes.InstanceProfile,
) (hasReferences bool, err error) {
	if ko.Spec.RoleRef != nil && ko.Spec.RoleRef.From != nil {
		hasReferences = true
		arr := ko.Spec.RoleRef.From
		if arr.Name == nil || *arr.Name == "" {
			return hasReferences, fmt.Errorf("provided resource reference is nil or empty: RoleRef")
		}
		namespace := ko.ObjectMeta.GetNamespace()
		if arr.Namespace != nil && *arr.Namespace != "" {
			namespace = *arr.Namespace
		}
		obj := &svcapitypes.Role{}
		if err := getReferencedResourceState_Role(ctx, apiReader, obj, *arr.Name, namespace); err != nil {
			return hasReferences, err
		}
		ko.Spec.Role = (*string)(obj.Spec.Name)
	}

	return hasReferences, nil
}

// getReferencedResourceState_Role looks up whether a referenced resource
// exists and is in a ACK.ResourceSynced=True state. If the referenced resource does exist and is
// in a Synced state, returns nil, otherwise returns `ackerr.ResourceReferenceTerminalFor` or
// `ResourceReferenceNotSyncedFor` depending on if the resource is in a Terminal state.
func getReferencedResourceState_Role(
	ctx context.Context,
	apiReader client.Reader,
	obj *svcapitypes.Role,
	name string, // the Kubernetes name of the referenced resource
	namespace string, // the Kubernetes namespace of the referenced resource
) error {
	namespacedName := types.NamespacedName{
		Namespace: namespace,
		Name:      name,
	}
	err := apiReader.Get(ctx, namespacedName, obj)
	if err != nil {
		return err
	}
	var refResourceTerminal bool
	for _, cond := range obj.Status.Conditions {
		if cond.Type == ackv1alpha1.ConditionTypeTerminal &&
			cond.Status == corev1.ConditionTrue {
			return ackerr.ResourceReferenceTerminalFor(
				"Role",
				namespace, name)
		}
	}
	if refResourceTerminal {
		return ackerr.ResourceReferenceTerminalFor(
			"Role",
			namespace, name)
	}
	var refResourceSynced bool
	for _, cond := range obj.Status.Conditions {
		if cond.Type == ackv1alpha1.ConditionTypeResourceSynced &&
			cond.Status == corev1.ConditionTrue {
			refResourceSynced = true
		}
	}
	if !refResourceSynced {
		return ackerr.ResourceReferenceNotSyncedFor(
			"Role",
			namespace, name)
	}
	if obj.Spec.Name == nil {
		return ackerr.ResourceReferenceMissingTargetFieldFor(
			"Role",
			namespace, name,
			"Spec.Name")
	}
	return nil
}
