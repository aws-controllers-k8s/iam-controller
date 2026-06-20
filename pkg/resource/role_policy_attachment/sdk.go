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
	"errors"
	"fmt"
	"reflect"

	ackv1alpha1 "github.com/aws-controllers-k8s/runtime/apis/core/v1alpha1"
	ackerr "github.com/aws-controllers-k8s/runtime/pkg/errors"
	ackrtlog "github.com/aws-controllers-k8s/runtime/pkg/runtime/log"
	"github.com/aws/aws-sdk-go-v2/aws"
	svcsdk "github.com/aws/aws-sdk-go-v2/service/iam"
	smithy "github.com/aws/smithy-go"
	corev1 "k8s.io/api/core/v1"

	svcapitypes "github.com/aws-controllers-k8s/iam-controller/apis/v1alpha1"
)

var (
	_ = &svcsdk.Client{}
	_ = &svcapitypes.RolePolicyAttachment{}
	_ = ackv1alpha1.AWSAccountID("")
	_ = &reflect.Value{}
	_ = fmt.Sprintf("")
	_ = &aws.Config{}
)

func (rm *resourceManager) sdkFind(ctx context.Context, r *resource) (latest *resource, err error) {
	rlog := ackrtlog.FromContext(ctx)
	exit := rlog.Trace("rm.sdkFind")
	defer func() { exit(err) }()
	if rm.requiredFieldsMissingFromReadOneInput(r) {
		return nil, ackerr.NotFound
	}
	attached, err := rm.attachmentExists(ctx, r.ko.Spec.RoleName, r.ko.Spec.PolicyARN)
	if err != nil {
		return nil, err
	}
	if !attached {
		return nil, ackerr.NotFound
	}
	ko := r.ko.DeepCopy()
	rm.setStatusDefaults(ko)
	ko.Status.Attached = aws.Bool(true)
	return &resource{ko}, nil
}

func (rm *resourceManager) requiredFieldsMissingFromReadOneInput(r *resource) bool {
	return r.ko.Spec.RoleName == nil || r.ko.Spec.PolicyARN == nil
}

func (rm *resourceManager) sdkCreate(ctx context.Context, desired *resource) (created *resource, err error) {
	rlog := ackrtlog.FromContext(ctx)
	exit := rlog.Trace("rm.sdkCreate")
	defer func() { exit(err) }()
	if desired.ko.Spec.RoleName == nil || desired.ko.Spec.PolicyARN == nil {
		return nil, ackerr.NewTerminalError(fmt.Errorf("roleName and policyARN are required"))
	}
	input := &svcsdk.AttachRolePolicyInput{RoleName: desired.ko.Spec.RoleName, PolicyArn: desired.ko.Spec.PolicyARN}
	_, err = rm.sdkapi.AttachRolePolicy(ctx, input)
	rm.metrics.RecordAPICall("CREATE", "AttachRolePolicy", err)
	if err != nil {
		return nil, err
	}
	return rm.sdkFind(ctx, desired)
}

func (rm *resourceManager) sdkUpdate(
	ctx context.Context,
	desired *resource,
	latest *resource,
	delta interface{},
) (updated *resource, err error) {
	if desired.ko.Spec.RoleName != nil && latest.ko.Spec.RoleName != nil && *desired.ko.Spec.RoleName != *latest.ko.Spec.RoleName {
		return nil, ackerr.NewTerminalError(fmt.Errorf("updates to spec.roleName are not supported"))
	}
	if desired.ko.Spec.PolicyARN != nil && latest.ko.Spec.PolicyARN != nil && *desired.ko.Spec.PolicyARN != *latest.ko.Spec.PolicyARN {
		return nil, ackerr.NewTerminalError(fmt.Errorf("updates to spec.policyARN are not supported"))
	}
	return rm.sdkFind(ctx, desired)
}

func (rm *resourceManager) sdkDelete(ctx context.Context, r *resource) (deleted *resource, err error) {
	rlog := ackrtlog.FromContext(ctx)
	exit := rlog.Trace("rm.sdkDelete")
	defer func() { exit(err) }()
	if r.ko.Spec.RoleName == nil || r.ko.Spec.PolicyARN == nil {
		return r, nil
	}
	input := &svcsdk.DetachRolePolicyInput{RoleName: r.ko.Spec.RoleName, PolicyArn: r.ko.Spec.PolicyARN}
	_, err = rm.sdkapi.DetachRolePolicy(ctx, input)
	rm.metrics.RecordAPICall("DELETE", "DetachRolePolicy", err)
	if err != nil {
		var apiErr smithy.APIError
		if errors.As(err, &apiErr) && apiErr.ErrorCode() == "NoSuchEntity" {
			return r, nil
		}
		return nil, err
	}
	ko := r.ko.DeepCopy()
	rm.setStatusDefaults(ko)
	ko.Status.Attached = aws.Bool(false)
	return &resource{ko}, nil
}

func (rm *resourceManager) setStatusDefaults(ko *svcapitypes.RolePolicyAttachment) {
	if ko.Status.ACKResourceMetadata == nil {
		ko.Status.ACKResourceMetadata = &ackv1alpha1.ResourceMetadata{}
	}
	if ko.Status.ACKResourceMetadata.Region == nil {
		ko.Status.ACKResourceMetadata.Region = &rm.awsRegion
	}
	if ko.Status.ACKResourceMetadata.Partition == nil {
		ko.Status.ACKResourceMetadata.Partition = &rm.awsPartition
	}
	if ko.Status.ACKResourceMetadata.OwnerAccountID == nil {
		ko.Status.ACKResourceMetadata.OwnerAccountID = &rm.awsAccountID
	}
	if ko.Status.Conditions == nil {
		ko.Status.Conditions = []*ackv1alpha1.Condition{}
	}
	if ko.Status.Attached == nil {
		ko.Status.Attached = aws.Bool(false)
	}
}

func (rm *resourceManager) updateConditions(r *resource, onSuccess bool, err error) (*resource, bool) {
	ko := r.ko.DeepCopy()
	rm.setStatusDefaults(ko)
	var terminalCondition *ackv1alpha1.Condition
	var recoverableCondition *ackv1alpha1.Condition
	var syncCondition *ackv1alpha1.Condition
	for _, condition := range ko.Status.Conditions {
		if condition.Type == ackv1alpha1.ConditionTypeTerminal {
			terminalCondition = condition
		}
		if condition.Type == ackv1alpha1.ConditionTypeRecoverable {
			recoverableCondition = condition
		}
		if condition.Type == ackv1alpha1.ConditionTypeResourceSynced {
			syncCondition = condition
		}
	}
	var termError *ackerr.TerminalError
	if rm.terminalAWSError(err) || errors.As(err, &termError) {
		if terminalCondition == nil {
			terminalCondition = &ackv1alpha1.Condition{Type: ackv1alpha1.ConditionTypeTerminal}
			ko.Status.Conditions = append(ko.Status.Conditions, terminalCondition)
		}
		errorMessage := err.Error()
		if awsErr, _ := ackerr.AWSError(err); awsErr != nil {
			errorMessage = awsErr.Error()
		}
		terminalCondition.Status = corev1.ConditionTrue
		terminalCondition.Message = &errorMessage
	} else {
		if terminalCondition != nil {
			terminalCondition.Status = corev1.ConditionFalse
			terminalCondition.Message = nil
		}
		if err != nil {
			if recoverableCondition == nil {
				recoverableCondition = &ackv1alpha1.Condition{Type: ackv1alpha1.ConditionTypeRecoverable}
				ko.Status.Conditions = append(ko.Status.Conditions, recoverableCondition)
			}
			recoverableCondition.Status = corev1.ConditionTrue
			errorMessage := err.Error()
			if awsErr, _ := ackerr.AWSError(err); awsErr != nil {
				errorMessage = awsErr.Error()
			}
			recoverableCondition.Message = &errorMessage
		} else if recoverableCondition != nil {
			recoverableCondition.Status = corev1.ConditionFalse
			recoverableCondition.Message = nil
		}
	}
	if syncCondition == nil {
		syncCondition = &ackv1alpha1.Condition{Type: ackv1alpha1.ConditionTypeResourceSynced}
		ko.Status.Conditions = append(ko.Status.Conditions, syncCondition)
	}
	if onSuccess && err == nil {
		syncCondition.Status = corev1.ConditionTrue
		syncCondition.Message = nil
	} else {
		syncCondition.Status = corev1.ConditionFalse
		if err != nil {
			errorMessage := err.Error()
			syncCondition.Message = &errorMessage
		} else {
			syncCondition.Message = nil
		}
	}
	return &resource{ko}, true
}

func (rm *resourceManager) terminalAWSError(err error) bool {
	if err == nil {
		return false
	}
	var terminalErr smithy.APIError
	if !errors.As(err, &terminalErr) {
		return false
	}
	switch terminalErr.ErrorCode() {
	case "InvalidInput", "MalformedPolicyDocument":
		return true
	default:
		return false
	}
}

func (rm *resourceManager) attachmentExists(ctx context.Context, roleName *string, policyARN *string) (bool, error) {
	input := &svcsdk.ListAttachedRolePoliciesInput{RoleName: roleName}
	paginator := svcsdk.NewListAttachedRolePoliciesPaginator(rm.sdkapi, input)
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			var apiErr smithy.APIError
			if errors.As(err, &apiErr) && apiErr.ErrorCode() == "NoSuchEntity" {
				return false, nil
			}
			return false, err
		}
		for _, attached := range page.AttachedPolicies {
			if attached.PolicyArn != nil && policyARN != nil && *attached.PolicyArn == *policyARN {
				return true, nil
			}
		}
	}
	rm.metrics.RecordAPICall("READ_MANY", "ListAttachedRolePolicies", nil)
	return false, nil
}
