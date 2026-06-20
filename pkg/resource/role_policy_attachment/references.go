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

func (rm *resourceManager) ClearResolvedReferences(res acktypes.AWSResource) acktypes.AWSResource {
	ko := rm.concreteResource(res).ko.DeepCopy()
	if ko.Spec.RoleRef != nil {
		ko.Spec.RoleName = nil
	}
	if ko.Spec.PolicyRef != nil {
		ko.Spec.PolicyARN = nil
	}
	return &resource{ko}
}

func (rm *resourceManager) ResolveReferences(
	ctx context.Context,
	apiReader client.Reader,
	res acktypes.AWSResource,
) (acktypes.AWSResource, bool, error) {
	ko := rm.concreteResource(res).ko
	resourceHasReferences := false
	if err := validateReferenceFields(ko); err != nil {
		return &resource{ko}, resourceHasReferences, err
	}
	if fieldHasReferences, err := rm.resolveReferenceForRole(ctx, apiReader, ko); err != nil {
		return &resource{ko}, (resourceHasReferences || fieldHasReferences), err
	} else {
		resourceHasReferences = resourceHasReferences || fieldHasReferences
	}
	if fieldHasReferences, err := rm.resolveReferenceForPolicy(ctx, apiReader, ko); err != nil {
		return &resource{ko}, (resourceHasReferences || fieldHasReferences), err
	} else {
		resourceHasReferences = resourceHasReferences || fieldHasReferences
	}
	return &resource{ko}, resourceHasReferences, nil
}

func validateReferenceFields(ko *svcapitypes.RolePolicyAttachment) error {
	if ko.Spec.RoleRef != nil && ko.Spec.RoleName != nil {
		return ackerr.ResourceReferenceAndIDNotSupportedFor("RoleName", "RoleRef")
	}
	if ko.Spec.PolicyRef != nil && ko.Spec.PolicyARN != nil {
		return ackerr.ResourceReferenceAndIDNotSupportedFor("PolicyARN", "PolicyRef")
	}
	return nil
}

func (rm *resourceManager) resolveReferenceForRole(
	ctx context.Context,
	apiReader client.Reader,
	ko *svcapitypes.RolePolicyAttachment,
) (hasReferences bool, err error) {
	if ko.Spec.RoleRef == nil || ko.Spec.RoleRef.From == nil {
		return false, nil
	}
	hasReferences = true
	arr := ko.Spec.RoleRef.From
	if arr.Name == nil || *arr.Name == "" {
		return hasReferences, fmt.Errorf("provided resource reference is nil or empty: RoleRef")
	}
	namespace := ko.GetNamespace()
	if arr.Namespace != nil && *arr.Namespace != "" {
		namespace = *arr.Namespace
	}
	obj := &svcapitypes.Role{}
	if err := getReferencedRole(ctx, apiReader, obj, *arr.Name, namespace); err != nil {
		return hasReferences, err
	}
	ko.Spec.RoleName = obj.Spec.Name
	return hasReferences, nil
}

func (rm *resourceManager) resolveReferenceForPolicy(
	ctx context.Context,
	apiReader client.Reader,
	ko *svcapitypes.RolePolicyAttachment,
) (hasReferences bool, err error) {
	if ko.Spec.PolicyRef == nil || ko.Spec.PolicyRef.From == nil {
		return false, nil
	}
	hasReferences = true
	arr := ko.Spec.PolicyRef.From
	if arr.Name == nil || *arr.Name == "" {
		return hasReferences, fmt.Errorf("provided resource reference is nil or empty: PolicyRef")
	}
	namespace := ko.GetNamespace()
	if arr.Namespace != nil && *arr.Namespace != "" {
		namespace = *arr.Namespace
	}
	obj := &svcapitypes.Policy{}
	if err := getReferencedPolicy(ctx, apiReader, obj, *arr.Name, namespace); err != nil {
		return hasReferences, err
	}
	if obj.Status.ACKResourceMetadata == nil || obj.Status.ACKResourceMetadata.ARN == nil {
		return hasReferences, ackerr.ResourceReferenceMissingTargetFieldFor("Policy", namespace, *arr.Name, "Status.ACKResourceMetadata.ARN")
	}
	policyARN := string(*obj.Status.ACKResourceMetadata.ARN)
	ko.Spec.PolicyARN = &policyARN
	return hasReferences, nil
}

func getReferencedRole(ctx context.Context, apiReader client.Reader, obj *svcapitypes.Role, name string, namespace string) error {
	namespacedName := types.NamespacedName{Namespace: namespace, Name: name}
	if err := apiReader.Get(ctx, namespacedName, obj); err != nil {
		return err
	}
	for _, cond := range obj.Status.Conditions {
		if cond.Type == ackv1alpha1.ConditionTypeTerminal && cond.Status == corev1.ConditionTrue {
			return ackerr.ResourceReferenceTerminalFor("Role", namespace, name)
		}
	}
	refResourceSynced := false
	for _, cond := range obj.Status.Conditions {
		if cond.Type == ackv1alpha1.ConditionTypeResourceSynced && cond.Status == corev1.ConditionTrue {
			refResourceSynced = true
		}
	}
	if !refResourceSynced {
		return ackerr.ResourceReferenceNotSyncedFor("Role", namespace, name)
	}
	if obj.Spec.Name == nil {
		return ackerr.ResourceReferenceMissingTargetFieldFor("Role", namespace, name, "Spec.Name")
	}
	return nil
}

func getReferencedPolicy(ctx context.Context, apiReader client.Reader, obj *svcapitypes.Policy, name string, namespace string) error {
	namespacedName := types.NamespacedName{Namespace: namespace, Name: name}
	if err := apiReader.Get(ctx, namespacedName, obj); err != nil {
		return err
	}
	for _, cond := range obj.Status.Conditions {
		if cond.Type == ackv1alpha1.ConditionTypeTerminal && cond.Status == corev1.ConditionTrue {
			return ackerr.ResourceReferenceTerminalFor("Policy", namespace, name)
		}
	}
	refResourceSynced := false
	for _, cond := range obj.Status.Conditions {
		if cond.Type == ackv1alpha1.ConditionTypeResourceSynced && cond.Status == corev1.ConditionTrue {
			refResourceSynced = true
		}
	}
	if !refResourceSynced {
		return ackerr.ResourceReferenceNotSyncedFor("Policy", namespace, name)
	}
	return nil
}
