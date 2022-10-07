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

package group

import (
	"context"

	ackrtlog "github.com/aws-controllers-k8s/runtime/pkg/runtime/log"
	ackutil "github.com/aws-controllers-k8s/runtime/pkg/util"
	svcsdk "github.com/aws/aws-sdk-go/service/iam"
)

// syncPolicies examines the PolicyARNs in the supplied Group and calls the
// ListGroupPolicies, AttachGroupPolicy and DetachGroupPolicy APIs to ensure
// that the set of attached policies stays in sync with the Group.Spec.Policies
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
		rlog.Debug("adding policy to group", "policy_arn", *p)
		if err = rm.addPolicy(ctx, r, p); err != nil {
			return err
		}
	}
	for _, p := range toDelete {
		rlog.Debug("removing policy from group", "policy_arn", *p)
		if err = rm.removePolicy(ctx, r, p); err != nil {
			return err
		}
	}

	return nil
}

// getPolicies returns the list of Policy ARNs currently attached to the Group
func (rm *resourceManager) getPolicies(
	ctx context.Context,
	r *resource,
) ([]*string, error) {
	var err error
	rlog := ackrtlog.FromContext(ctx)
	exit := rlog.Trace("rm.getPolicies")
	defer exit(err)

	input := &svcsdk.ListAttachedGroupPoliciesInput{}
	input.GroupName = r.ko.Spec.Name
	res := []*string{}

	err = rm.sdkapi.ListAttachedGroupPoliciesPagesWithContext(
		ctx, input, func(page *svcsdk.ListAttachedGroupPoliciesOutput, _ bool) bool {
			if page == nil {
				return true
			}
			for _, p := range page.AttachedPolicies {
				res = append(res, p.PolicyArn)
			}
			return page.IsTruncated != nil && *page.IsTruncated
		},
	)
	rm.metrics.RecordAPICall("GET", "ListAttachedGroupPolicies", err)
	return res, err
}

// addPolicy adds the supplied Policy to the supplied Group resource
func (rm *resourceManager) addPolicy(
	ctx context.Context,
	r *resource,
	policyARN *string,
) (err error) {
	rlog := ackrtlog.FromContext(ctx)
	exit := rlog.Trace("rm.addPolicy")
	defer exit(err)

	input := &svcsdk.AttachGroupPolicyInput{}
	input.GroupName = r.ko.Spec.Name
	input.PolicyArn = policyARN
	_, err = rm.sdkapi.AttachGroupPolicyWithContext(ctx, input)
	rm.metrics.RecordAPICall("CREATE", "AttachGroupPolicy", err)
	return err
}

// removePolicy removes the supplied Policy from the supplied Group resource
func (rm *resourceManager) removePolicy(
	ctx context.Context,
	r *resource,
	policyARN *string,
) (err error) {
	rlog := ackrtlog.FromContext(ctx)
	exit := rlog.Trace("rm.removePolicy")
	defer exit(err)

	input := &svcsdk.DetachGroupPolicyInput{}
	input.GroupName = r.ko.Spec.Name
	input.PolicyArn = policyARN
	_, err = rm.sdkapi.DetachGroupPolicyWithContext(ctx, input)
	rm.metrics.RecordAPICall("DELETE", "DetachGroupPolicy", err)
	return err
}
