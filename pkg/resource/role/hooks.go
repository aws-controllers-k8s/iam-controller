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
	"net/url"

	ackrtlog "github.com/aws-controllers-k8s/runtime/pkg/runtime/log"
	ackutil "github.com/aws-controllers-k8s/runtime/pkg/util"
	svcsdk "github.com/aws/aws-sdk-go/service/iam"

	svcapitypes "github.com/aws-controllers-k8s/iam-controller/apis/v1alpha1"
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
		rlog.Debug("adding policy to role", "policy_arn", *p)
		if err = rm.addPolicy(ctx, r, p); err != nil {
			return err
		}
	}
	for _, p := range toDelete {
		rlog.Debug("removing policy from role", "policy_arn", *p)
		if err = rm.removePolicy(ctx, r, p); err != nil {
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

// addPolicy adds the supplied Policy to the supplied Role resource
func (rm *resourceManager) addPolicy(
	ctx context.Context,
	r *resource,
	policyARN *string,
) (err error) {
	rlog := ackrtlog.FromContext(ctx)
	exit := rlog.Trace("rm.addPolicy")
	defer exit(err)

	input := &svcsdk.AttachRolePolicyInput{}
	input.RoleName = r.ko.Spec.Name
	input.PolicyArn = policyARN
	_, err = rm.sdkapi.AttachRolePolicyWithContext(ctx, input)
	rm.metrics.RecordAPICall("CREATE", "AttachRolePolicy", err)
	return err
}

// removePolicy removes the supplied Policy from the supplied Role resource
func (rm *resourceManager) removePolicy(
	ctx context.Context,
	r *resource,
	policyARN *string,
) (err error) {
	rlog := ackrtlog.FromContext(ctx)
	exit := rlog.Trace("rm.removePolicy")
	defer exit(err)

	input := &svcsdk.DetachRolePolicyInput{}
	input.RoleName = r.ko.Spec.Name
	input.PolicyArn = policyARN
	_, err = rm.sdkapi.DetachRolePolicyWithContext(ctx, input)
	rm.metrics.RecordAPICall("DELETE", "DetachRolePolicy", err)
	return err
}

// syncTags examines the Tags in the supplied Role and calls the ListRoleTags,
// TagRole and UntagRole APIs to ensure that the set of associated Tags  stays
// in sync with the Role.Spec.Tags
func (rm *resourceManager) syncTags(
	ctx context.Context,
	r *resource,
) (err error) {
	rlog := ackrtlog.FromContext(ctx)
	exit := rlog.Trace("rm.syncTags")
	defer exit(err)
	toAdd := []*svcapitypes.Tag{}
	toDelete := []*svcapitypes.Tag{}

	existingTags, err := rm.getTags(ctx, r)
	if err != nil {
		return err
	}

	for _, t := range r.ko.Spec.Tags {
		if !inTags(*t.Key, *t.Value, existingTags) {
			toAdd = append(toAdd, t)
		}
	}

	for _, t := range existingTags {
		if !inTags(*t.Key, *t.Value, r.ko.Spec.Tags) {
			toDelete = append(toDelete, t)
		}
	}

	if len(toAdd) > 0 {
		for _, t := range toAdd {
			rlog.Debug("adding tag to role", "key", *t.Key, "value", *t.Value)
		}
		if err = rm.addTags(ctx, r, toAdd); err != nil {
			return err
		}
	}
	if len(toDelete) > 0 {
		for _, t := range toDelete {
			rlog.Debug("removing tag from role", "key", *t.Key, "value", *t.Value)
		}
		if err = rm.removeTags(ctx, r, toDelete); err != nil {
			return err
		}
	}

	return nil
}

// inTags returns true if the supplied key and value can be found in the
// supplied list of Tag structs.
//
// TODO(jaypipes): When we finally standardize Tag handling in ACK, move this
// to the ACK common runtime/ or pkg/ repos
func inTags(
	key string,
	value string,
	tags []*svcapitypes.Tag,
) bool {
	for _, t := range tags {
		if *t.Key == key && *t.Value == value {
			return true
		}
	}
	return false
}

// getTags returns the list of tags to the Role
func (rm *resourceManager) getTags(
	ctx context.Context,
	r *resource,
) ([]*svcapitypes.Tag, error) {
	var err error
	var resp *svcsdk.ListRoleTagsOutput
	rlog := ackrtlog.FromContext(ctx)
	exit := rlog.Trace("rm.getTags")
	defer exit(err)

	input := &svcsdk.ListRoleTagsInput{}
	input.RoleName = r.ko.Spec.Name
	res := []*svcapitypes.Tag{}

	for {
		resp, err = rm.sdkapi.ListRoleTagsWithContext(ctx, input)
		if err != nil || resp == nil {
			break
		}
		for _, t := range resp.Tags {
			res = append(res, &svcapitypes.Tag{Key: t.Key, Value: t.Value})
		}
		if resp.IsTruncated != nil && !*resp.IsTruncated {
			break
		}
	}
	rm.metrics.RecordAPICall("GET", "ListRoleTags", err)
	return res, err
}

// addTags adds the supplied Tags to the supplied Role resource
func (rm *resourceManager) addTags(
	ctx context.Context,
	r *resource,
	tags []*svcapitypes.Tag,
) (err error) {
	rlog := ackrtlog.FromContext(ctx)
	exit := rlog.Trace("rm.addTag")
	defer exit(err)

	input := &svcsdk.TagRoleInput{}
	input.RoleName = r.ko.Spec.Name
	inTags := []*svcsdk.Tag{}
	for _, t := range tags {
		inTags = append(inTags, &svcsdk.Tag{Key: t.Key, Value: t.Value})
	}
	input.Tags = inTags

	_, err = rm.sdkapi.TagRoleWithContext(ctx, input)
	rm.metrics.RecordAPICall("CREATE", "TagRole", err)
	return err
}

// removeTags removes the supplied Tags from the supplied Role resource
func (rm *resourceManager) removeTags(
	ctx context.Context,
	r *resource,
	tags []*svcapitypes.Tag,
) (err error) {
	rlog := ackrtlog.FromContext(ctx)
	exit := rlog.Trace("rm.removeTag")
	defer exit(err)

	input := &svcsdk.UntagRoleInput{}
	input.RoleName = r.ko.Spec.Name
	inTagKeys := []*string{}
	for _, t := range tags {
		inTagKeys = append(inTagKeys, t.Key)
	}
	input.TagKeys = inTagKeys

	_, err = rm.sdkapi.UntagRoleWithContext(ctx, input)
	rm.metrics.RecordAPICall("DELETE", "UntagRole", err)
	return err
}

func decodeAssumeDocument(encoded string) (string, error) {
	return url.QueryUnescape(encoded)
}
