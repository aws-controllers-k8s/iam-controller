package service_linked_role

import (
	"context"
	"errors"

	svcapitypes "github.com/aws-controllers-k8s/iam-controller/apis/v1alpha1"
	ackv1alpha1 "github.com/aws-controllers-k8s/runtime/apis/core/v1alpha1"
	ackcompare "github.com/aws-controllers-k8s/runtime/pkg/compare"
	ackerr "github.com/aws-controllers-k8s/runtime/pkg/errors"
	ackrtlog "github.com/aws-controllers-k8s/runtime/pkg/runtime/log"
	svcsdk "github.com/aws/aws-sdk-go-v2/service/iam"
	"github.com/aws/smithy-go"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (rm *resourceManager) customGetServiceLinkedRole(
	ctx context.Context,
	r *resource,
) (latest *resource, err error) {
	rlog := ackrtlog.FromContext(ctx)
	exit := rlog.Trace("rm.customGetServiceLinkedRole")
	defer func() {
		exit(err)
	}()

	// If any required fields in the input shape are missing, AWS resource is
	// not created yet. Return NotFound here to indicate to callers that the
	// resource isn't yet created.
	if rm.requiredFieldsMissingFromReadOneInput(r) {
		return nil, ackerr.NotFound
	}

	input, err := rm.newDescribeRequestPayload(r)
	if err != nil {
		return nil, err
	}

	var resp *svcsdk.GetRoleOutput
	resp, err = rm.sdkapi.GetRole(ctx, input)
	rm.metrics.RecordAPICall("READ_ONE", "GetRole", err)
	if err != nil {
		var awsErr smithy.APIError
		if errors.As(err, &awsErr) && awsErr.ErrorCode() == "NoSuchEntity" {
			return nil, ackerr.NotFound
		}
		return nil, err
	}

	// Merge in the information we read from the API call above to the copy of
	// the original Kubernetes object we passed to the function
	ko := r.ko.DeepCopy()

	if ko.Status.ACKResourceMetadata == nil {
		ko.Status.ACKResourceMetadata = &ackv1alpha1.ResourceMetadata{}
	}
	if resp.Role.Arn != nil {
		arn := ackv1alpha1.AWSResourceName(*resp.Role.Arn)
		ko.Status.ACKResourceMetadata.ARN = &arn
	}
	if resp.Role.CreateDate != nil {
		ko.Status.CreateDate = &metav1.Time{*resp.Role.CreateDate}
	} else {
		ko.Status.CreateDate = nil
	}
	if resp.Role.Description != nil {
		ko.Spec.Description = resp.Role.Description
	} else {
		ko.Spec.Description = nil
	}
	if resp.Role.MaxSessionDuration != nil {
		maxSessionDurationCopy := int64(*resp.Role.MaxSessionDuration)
		ko.Status.MaxSessionDuration = &maxSessionDurationCopy
	} else {
		ko.Status.MaxSessionDuration = nil
	}
	if resp.Role.RoleId != nil {
		ko.Status.RoleID = resp.Role.RoleId
	} else {
		ko.Status.RoleID = nil
	}
	if resp.Role.RoleLastUsed != nil {
		f8 := &svcapitypes.RoleLastUsed{}
		if resp.Role.RoleLastUsed.LastUsedDate != nil {
			f8.LastUsedDate = &metav1.Time{*resp.Role.RoleLastUsed.LastUsedDate}
		}
		if resp.Role.RoleLastUsed.Region != nil {
			f8.Region = resp.Role.RoleLastUsed.Region
		}
		ko.Status.RoleLastUsed = f8
	} else {
		ko.Status.RoleLastUsed = nil
	}
	if resp.Role.RoleName != nil {
		ko.Status.RoleName = resp.Role.RoleName
	} else {
		ko.Status.RoleName = nil
	}

	rm.setStatusDefaults(ko)

	return &resource{ko}, nil
}

// newDescribeRequestPayload returns SDK-specific struct for the HTTP request
// payload of the Describe API call for the resource
func (rm *resourceManager) newDescribeRequestPayload(
	r *resource,
) (*svcsdk.GetRoleInput, error) {
	res := &svcsdk.GetRoleInput{}

	if r.ko.Spec.AWSServiceName != nil {
		res.RoleName = r.ko.Status.RoleName
	}

	return res, nil
}

// requiredFieldsMissingFromReadOneInput returns true if there are any fields
// for the ReadOne Input shape that are required but not present in the
// resource's Spec or Status
func (rm *resourceManager) requiredFieldsMissingFromReadOneInput(
	r *resource,
) bool {
	// for service linked roles we get the roleName from the status
	return r.ko.Status.RoleName == nil

}

// sdkUpdate patches the supplied resource in the backend AWS service API and
// returns a new resource with updated fields.
func (rm *resourceManager) customUpdateServiceLinkedRole(
	ctx context.Context,
	desired *resource,
	latest *resource,
	delta *ackcompare.Delta,
) (updated *resource, err error) {
	rlog := ackrtlog.FromContext(ctx)
	exit := rlog.Trace("rm.customUpdateServiceLinkedRole")
	defer func() {
		exit(err)
	}()

	input, err := rm.newUpdateRequestPayload(ctx, desired, delta)
	if err != nil {
		return nil, err
	}

	var resp *svcsdk.UpdateRoleOutput
	_ = resp
	resp, err = rm.sdkapi.UpdateRole(ctx, input)
	rm.metrics.RecordAPICall("UPDATE", "UpdateRole", err)
	if err != nil {
		return nil, err
	}
	// Merge in the information we read from the API call above to the copy of
	// the original Kubernetes object we passed to the function
	ko := desired.ko.DeepCopy()

	rm.setStatusDefaults(ko)
	return &resource{ko}, nil
}

// newUpdateRequestPayload returns an SDK-specific struct for the HTTP request
// payload of the Update API call for the resource
func (rm *resourceManager) newUpdateRequestPayload(
	ctx context.Context,
	r *resource,
	delta *ackcompare.Delta,
) (*svcsdk.UpdateRoleInput, error) {
	res := &svcsdk.UpdateRoleInput{}

	res.RoleName = r.ko.Status.RoleName

	if r.ko.Spec.Description != nil {
		res.Description = r.ko.Spec.Description
	}

	return res, nil
}
