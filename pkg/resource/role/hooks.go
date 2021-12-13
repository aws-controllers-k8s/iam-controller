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
	"context"

	ackrtlog "github.com/aws-controllers-k8s/runtime/pkg/runtime/log"
	ackutil "github.com/aws-controllers-k8s/runtime/pkg/util"
	svcsdk "github.com/aws/aws-sdk-go/service/iam"
)

// syncPolicies examines the PolicyARNs in the supplied Role and calls the
// ListRolePolicies, AttachRolePolicy and DetachRolePolicy APIs to ensure that
// the set of attached policies stays in sync with the Role.Spec.Policies
// field, which is a list of strings containing Policy ARNs.
func (rm *resourceManager) syncPolicies(
	ctx context.Context,
	r *resource,
) (err error) {
	rlog := ackrtlog.FromContext(ctx)
	exit := rlog.Trace("rm.syncPolicies")
	defer exit(err)
	toAdd := []*string{}
	toDelete := []*string{}

	existingPolicies, err := rm.getPolicies(ctx, r)
	if err != nil {
		return err
	}

	for _, p := range r.ko.Spec.Policies {
		if !ackutil.InStringPs(*p, existingPolicies) {
			toAdd = append(toAdd, p)
		}
	}

	for _, p := range existingPolicies {
		if !ackutil.InStringPs(*p, r.ko.Spec.Policies) {
			toDelete = append(toDelete, p)
		}
	}

	for _, p := range toAdd {
		rlog.Debug("attaching policy to role", "policy_arn", *p)
		if err = rm.attachPolicy(ctx, r, p); err != nil {
			return err
		}
	}
	for _, p := range toDelete {
		rlog.Debug("detaching policy from role", "policy_arn", *p)
		if err = rm.detachPolicy(ctx, r, p); err != nil {
			return err
		}
	}

	return nil
}

// getPolicies returns the list of Policy ARNs currently attached to the Role
func (rm *resourceManager) getPolicies(
	ctx context.Context,
	r *resource,
) ([]*string, error) {
	var err error
	rlog := ackrtlog.FromContext(ctx)
	exit := rlog.Trace("rm.getPolicies")
	defer exit(err)

	input := &svcsdk.ListAttachedRolePoliciesInput{}
	input.RoleName = r.ko.Spec.Name
	res := []*string{}

	err = rm.sdkapi.ListAttachedRolePoliciesPagesWithContext(
		ctx, input, func(page *svcsdk.ListAttachedRolePoliciesOutput, _ bool) bool {
			if page == nil {
				return true
			}
			for _, p := range page.AttachedPolicies {
				res = append(res, p.PolicyArn)
			}
			return page.IsTruncated != nil && *page.IsTruncated
		},
	)
	rm.metrics.RecordAPICall("GET", "ListAttachedRolePolicies", err)
	return res, err
}

// attachPolicy attaches the supplied Policy to the supplied Role resource
func (rm *resourceManager) attachPolicy(
	ctx context.Context,
	r *resource,
	policyARN *string,
) (err error) {
	rlog := ackrtlog.FromContext(ctx)
	exit := rlog.Trace("rm.attachPolicy")
	defer exit(err)

	input := &svcsdk.AttachRolePolicyInput{}
	input.RoleName = r.ko.Spec.Name
	input.PolicyArn = policyARN
	_, err = rm.sdkapi.AttachRolePolicyWithContext(ctx, input)
	rm.metrics.RecordAPICall("CREATE", "AttachRolePolicy", err)
	return err
}

// detachPolicy detaches the supplied Policy from the supplied Role resource
func (rm *resourceManager) detachPolicy(
	ctx context.Context,
	r *resource,
	policyARN *string,
) (err error) {
	rlog := ackrtlog.FromContext(ctx)
	exit := rlog.Trace("rm.detachPolicy")
	defer exit(err)

	input := &svcsdk.DetachRolePolicyInput{}
	input.RoleName = r.ko.Spec.Name
	input.PolicyArn = policyARN
	_, err = rm.sdkapi.DetachRolePolicyWithContext(ctx, input)
	rm.metrics.RecordAPICall("DELETE", "DetachRolePolicy", err)
	return err
}
