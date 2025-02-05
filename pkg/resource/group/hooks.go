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
	"net/url"

	ackrtlog "github.com/aws-controllers-k8s/runtime/pkg/runtime/log"
	ackutil "github.com/aws-controllers-k8s/runtime/pkg/util"
	svcsdk "github.com/aws/aws-sdk-go-v2/service/iam"
	"github.com/samber/lo"

	commonutil "github.com/aws-controllers-k8s/iam-controller/pkg/util"
)

// syncManagedPolicies examines the managed PolicyARNs in the supplied Group
// and calls the ListAttachedGroupPolicies, AttachGroupPolicy and
// DetachGroupPolicy APIs to ensure that the set of attached policies stays in
// sync with the Group.Spec.Policies field, which is a list of strings
// containing Policy ARNs.
func (rm *resourceManager) syncManagedPolicies(
	ctx context.Context,
	desired *resource,
	latest *resource,
) (err error) {
	rlog := ackrtlog.FromContext(ctx)
	exit := rlog.Trace("rm.syncManagedPolicies")
	defer func() { exit(err) }()
	toAdd := []*string{}
	toDelete := []*string{}

	existingPolicies := latest.ko.Spec.Policies

	for _, p := range desired.ko.Spec.Policies {
		if !ackutil.InStringPs(*p, existingPolicies) {
			toAdd = append(toAdd, p)
		}
	}

	for _, p := range existingPolicies {
		if !ackutil.InStringPs(*p, desired.ko.Spec.Policies) {
			toDelete = append(toDelete, p)
		}
	}

	for _, p := range toAdd {
		rlog.Debug("adding managed policy to group", "policy_arn", *p)
		if err = rm.addManagedPolicy(ctx, desired, p); err != nil {
			return err
		}
	}
	for _, p := range toDelete {
		rlog.Debug("removing managed policy from group", "policy_arn", *p)
		if err = rm.removeManagedPolicy(ctx, desired, p); err != nil {
			return err
		}
	}

	return nil
}

// getManagedPolicies returns the list of managed Policy ARNs currently
// attached to the Group
func (rm *resourceManager) getManagedPolicies(
	ctx context.Context,
	r *resource,
) ([]*string, error) {
	var err error
	rlog := ackrtlog.FromContext(ctx)
	exit := rlog.Trace("rm.getManagedPolicies")
	defer func() { exit(err) }()

	input := &svcsdk.ListAttachedGroupPoliciesInput{}
	input.GroupName = r.ko.Spec.Name
	res := []*string{}

	paginator := svcsdk.NewListAttachedGroupPoliciesPaginator(rm.sdkapi, input)
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		for _, p := range page.AttachedPolicies {
			res = append(res, p.PolicyArn)
		}
	}
	rm.metrics.RecordAPICall("READ_MANY", "ListAttachedGroupPolicies", err)
	return res, err
}

// addManagedPolicy adds the supplied managed Policy to the supplied Group
// resource
func (rm *resourceManager) addManagedPolicy(
	ctx context.Context,
	r *resource,
	policyARN *string,
) (err error) {
	rlog := ackrtlog.FromContext(ctx)
	exit := rlog.Trace("rm.addManagedPolicy")
	defer func() { exit(err) }()

	input := &svcsdk.AttachGroupPolicyInput{}
	input.GroupName = r.ko.Spec.Name
	input.PolicyArn = policyARN
	_, err = rm.sdkapi.AttachGroupPolicy(ctx, input)
	rm.metrics.RecordAPICall("UPDATE", "AttachGroupPolicy", err)
	return err
}

// removeManagedPolicy removes the supplied managed Policy from the supplied
// Group resource
func (rm *resourceManager) removeManagedPolicy(
	ctx context.Context,
	r *resource,
	policyARN *string,
) (err error) {
	rlog := ackrtlog.FromContext(ctx)
	exit := rlog.Trace("rm.removeManagedPolicy")
	defer func() { exit(err) }()

	input := &svcsdk.DetachGroupPolicyInput{}
	input.GroupName = r.ko.Spec.Name
	input.PolicyArn = policyARN
	_, err = rm.sdkapi.DetachGroupPolicy(ctx, input)
	rm.metrics.RecordAPICall("UPDATE", "DetachGroupPolicy", err)
	return err
}

// syncInlinePolicies examines the InlinePolicies in the supplied Group and
// calls the ListGroupPolicies, PutGroupPolicy and DeleteGroupPolicy APIs to
// ensure that the set of attached policies stays in sync with the
// Group.Spec.InlinePolicies field, which is a map of policy names to policy
// documents.
func (rm *resourceManager) syncInlinePolicies(
	ctx context.Context,
	desired *resource,
	latest *resource,
) (err error) {
	rlog := ackrtlog.FromContext(ctx)
	exit := rlog.Trace("rm.syncInlinePolicies")
	defer func() { exit(err) }()

	existingPolicies := latest.ko.Spec.InlinePolicies

	existingPairs := lo.ToPairs(
		commonutil.MapStringFromMapStringPointers(existingPolicies),
	)
	desiredPairs := lo.ToPairs(
		commonutil.MapStringFromMapStringPointers(desired.ko.Spec.InlinePolicies),
	)

	toDelete, toAdd := lo.Difference(existingPairs, desiredPairs)

	for _, pair := range toAdd {
		polName := pair.Key
		polDoc := pair.Value
		rlog.Debug(
			"adding inline policy to group",
			"policy_name", polName,
		)
		err = rm.addInlinePolicy(ctx, desired, polName, &polDoc)
		if err != nil {
			return err
		}
	}
	for _, pair := range toDelete {
		// do not remove elements we just updated with `addInlinePolicy`
		if _, ok := lo.Find(toAdd, func(entry lo.Entry[string, string]) bool { return entry.Key == pair.Key }); ok {
			continue
		}

		polName := pair.Key
		rlog.Debug(
			"removing inline policy from group",
			"policy_name", polName,
		)
		if err = rm.removeInlinePolicy(ctx, desired, polName); err != nil {
			return err
		}
	}

	return nil
}

// getInlinePolicies returns a map of inline policy name and policy docs
// currently attached to the Group.
//
// NOTE(jaypipes): There's no way around the inefficiencies of this method
// without caching stuff, and I don't think it's useful to have an unbounded
// cache for these inline policy documents :( IAM's ListGroupPolicies API call
// only returns the *policy names* of inline policies. You need to call
// GetGroupPolicy API call for each inline policy name in order to get the
// policy document. Yes, they force an O(N) time complexity for this
// operation...
func (rm *resourceManager) getInlinePolicies(
	ctx context.Context,
	r *resource,
) (map[string]*string, error) {
	var err error
	rlog := ackrtlog.FromContext(ctx)
	exit := rlog.Trace("rm.getInlinePolicies")
	defer func() { exit(err) }()

	groupName := r.ko.Spec.Name

	input := &svcsdk.ListGroupPoliciesInput{}
	input.GroupName = groupName
	res := map[string]*string{}

	paginator := svcsdk.NewListGroupPoliciesPaginator(rm.sdkapi, input)
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		for _, p := range page.PolicyNames {
			res[p] = nil
		}
	}
	rm.metrics.RecordAPICall("READ_MANY", "ListGroupPolicies", err)

	// Now we need to grab the policy documents for each policy name
	for polName, _ := range res {
		input := &svcsdk.GetGroupPolicyInput{}
		input.GroupName = groupName
		input.PolicyName = &polName
		resp, err := rm.sdkapi.GetGroupPolicy(ctx, input)
		rm.metrics.RecordAPICall("READ_ONE", "GetGroupPolicy", err)
		if err != nil {
			return nil, err
		}
		cleanedDoc, err := decodeDocument(*resp.PolicyDocument)
		if err != nil {
			return nil, err
		}
		res[polName] = &cleanedDoc

	}
	return res, nil
}

// addInlinePolicy adds the supplied inline Policy to the supplied Group
// resource
func (rm *resourceManager) addInlinePolicy(
	ctx context.Context,
	r *resource,
	policyName string,
	policyDoc *string,
) (err error) {
	rlog := ackrtlog.FromContext(ctx)
	exit := rlog.Trace("rm.addInlinePolicy")
	defer func() { exit(err) }()

	input := &svcsdk.PutGroupPolicyInput{}
	input.GroupName = r.ko.Spec.Name
	input.PolicyName = &policyName
	cleanedDoc, err := decodeDocument(*policyDoc)
	if err != nil {
		return err
	}
	input.PolicyDocument = &cleanedDoc
	_, err = rm.sdkapi.PutGroupPolicy(ctx, input)
	rm.metrics.RecordAPICall("UPDATE", "PutGroupPolicy", err)
	return err
}

// removeInlinePolicy removes the supplied inline Policy from the supplied
// Group resource
func (rm *resourceManager) removeInlinePolicy(
	ctx context.Context,
	r *resource,
	policyName string,
) (err error) {
	rlog := ackrtlog.FromContext(ctx)
	exit := rlog.Trace("rm.removeInlinePolicy")
	defer func() { exit(err) }()

	input := &svcsdk.DeleteGroupPolicyInput{}
	input.GroupName = r.ko.Spec.Name
	input.PolicyName = &policyName
	_, err = rm.sdkapi.DeleteGroupPolicy(ctx, input)
	rm.metrics.RecordAPICall("UPDATE", "DeleteGroupPolicy", err)
	return err
}

func decodeDocument(encoded string) (string, error) {
	return url.QueryUnescape(encoded)
}
