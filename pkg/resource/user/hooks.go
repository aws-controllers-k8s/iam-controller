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

package user

import (
	"context"

	ackcompare "github.com/aws-controllers-k8s/runtime/pkg/compare"
	ackrtlog "github.com/aws-controllers-k8s/runtime/pkg/runtime/log"
	ackutil "github.com/aws-controllers-k8s/runtime/pkg/util"
	svcsdk "github.com/aws/aws-sdk-go/service/iam"

	svcapitypes "github.com/aws-controllers-k8s/iam-controller/apis/v1alpha1"
	commonutil "github.com/aws-controllers-k8s/iam-controller/pkg/util"
)

// putUserPermissionsBoundary calls the IAM API to set a given user
// permission boundary.
func (rm *resourceManager) putUserPermissionsBoundary(
	ctx context.Context,
	r *resource,
) (err error) {
	rlog := ackrtlog.FromContext(ctx)
	exit := rlog.Trace("rm.putUserPermissionsBoundary")
	defer func() { exit(err) }()

	input := &svcsdk.PutUserPermissionsBoundaryInput{
		UserName:            r.ko.Spec.Name,
		PermissionsBoundary: r.ko.Spec.PermissionsBoundary,
	}
	_, err = rm.sdkapi.PutUserPermissionsBoundaryWithContext(ctx, input)
	rm.metrics.RecordAPICall("UPDATE", "PutUserPermissionsBoundary", err)
	return err
}

// deleteUserPermissionsBoundary calls the IAM API to delete a given user
// permission boundary.
func (rm *resourceManager) deleteUserPermissionsBoundary(
	ctx context.Context,
	r *resource,
) (err error) {
	rlog := ackrtlog.FromContext(ctx)
	exit := rlog.Trace("rm.deleteUserPermissionsBoundary")
	defer func() { exit(err) }()

	input := &svcsdk.DeleteUserPermissionsBoundaryInput{UserName: r.ko.Spec.Name}
	_, err = rm.sdkapi.DeleteUserPermissionsBoundaryWithContext(ctx, input)
	rm.metrics.RecordAPICall("UPDATE", "DeleteUserPermissionsBoundary", err)
	return err
}

// syncUserPermissionsBoundary synchronises user permissions boundary
func (rm *resourceManager) syncUserPermissionsBoundary(
	ctx context.Context,
	r *resource,
) (err error) {
	rlog := ackrtlog.FromContext(ctx)
	exit := rlog.Trace("rm.syncUserPermissionsBoundary")
	defer func() { exit(err) }()
	if r.ko.Spec.PermissionsBoundary == nil || *r.ko.Spec.PermissionsBoundary == "" {
		return rm.deleteUserPermissionsBoundary(ctx, r)
	}
	return rm.putUserPermissionsBoundary(ctx, r)
}

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
		rlog.Debug("adding policy to user", "policy_arn", *p)
		if err = rm.addPolicy(ctx, r, p); err != nil {
			return err
		}
	}
	for _, p := range toDelete {
		rlog.Debug("removing policy from user", "policy_arn", *p)
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
	defer func() {
		exit(err)
	}()

	input := &svcsdk.ListAttachedUserPoliciesInput{}
	input.UserName = r.ko.Spec.Name
	res := []*string{}

	err = rm.sdkapi.ListAttachedUserPoliciesPagesWithContext(
		ctx, input, func(page *svcsdk.ListAttachedUserPoliciesOutput, _ bool) bool {
			if page == nil {
				return true
			}
			for _, p := range page.AttachedPolicies {
				res = append(res, p.PolicyArn)
			}
			return page.IsTruncated != nil && *page.IsTruncated
		},
	)
	rm.metrics.RecordAPICall("GET", "ListAttachedUserPolicies", err)
	return res, err
}

// addPolicy adds the supplied Policy to the supplied User resource
func (rm *resourceManager) addPolicy(
	ctx context.Context,
	r *resource,
	policyARN *string,
) (err error) {
	rlog := ackrtlog.FromContext(ctx)
	exit := rlog.Trace("rm.addPolicy")
	defer func() {
		exit(err)
	}()

	input := &svcsdk.AttachUserPolicyInput{}
	input.UserName = r.ko.Spec.Name
	input.PolicyArn = policyARN
	_, err = rm.sdkapi.AttachUserPolicyWithContext(ctx, input)
	rm.metrics.RecordAPICall("UPDATE", "AttachUserPolicy", err)
	return err
}

// removePolicy removes the supplied Policy from the supplied User resource
func (rm *resourceManager) removePolicy(
	ctx context.Context,
	r *resource,
	policyARN *string,
) (err error) {
	rlog := ackrtlog.FromContext(ctx)
	exit := rlog.Trace("rm.removePolicy")
	defer func() {
		exit(err)
	}()

	input := &svcsdk.DetachUserPolicyInput{}
	input.UserName = r.ko.Spec.Name
	input.PolicyArn = policyARN
	_, err = rm.sdkapi.DetachUserPolicyWithContext(ctx, input)
	rm.metrics.RecordAPICall("UPDATE", "DetachUserPolicy", err)
	return err
}

// compareTags is a custom comparison function for comparing lists of Tag
// structs where the order of the structs in the list is not important.
func compareTags(
	delta *ackcompare.Delta,
	a *resource,
	b *resource,
) {
	if len(a.ko.Spec.Tags) != len(b.ko.Spec.Tags) {
		delta.Add("Spec.Tags", a.ko.Spec.Tags, b.ko.Spec.Tags)
	} else if len(a.ko.Spec.Tags) > 0 {
		if !commonutil.EqualTags(a.ko.Spec.Tags, b.ko.Spec.Tags) {
			delta.Add("Spec.Tags", a.ko.Spec.Tags, b.ko.Spec.Tags)
		}
	}
}

// syncTags examines the Tags in the supplied User and calls the ListUserTags,
// TagUser and UntagUser APIs to ensure that the set of associated Tags  stays
// in sync with the User.Spec.Tags
func (rm *resourceManager) syncTags(
	ctx context.Context,
	r *resource,
) (err error) {
	rlog := ackrtlog.FromContext(ctx)
	exit := rlog.Trace("rm.syncTags")
	defer func() {
		exit(err)
	}()
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
			rlog.Debug("adding tag to user", "key", *t.Key, "value", *t.Value)
		}
		if err = rm.addTags(ctx, r, toAdd); err != nil {
			return err
		}
	}
	if len(toDelete) > 0 {
		for _, t := range toDelete {
			rlog.Debug("removing tag from user", "key", *t.Key, "value", *t.Value)
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

// getTags returns the list of tags to the User
func (rm *resourceManager) getTags(
	ctx context.Context,
	r *resource,
) ([]*svcapitypes.Tag, error) {
	var err error
	var resp *svcsdk.ListUserTagsOutput
	rlog := ackrtlog.FromContext(ctx)
	exit := rlog.Trace("rm.getTags")
	defer func() {
		exit(err)
	}()

	input := &svcsdk.ListUserTagsInput{}
	input.UserName = r.ko.Spec.Name
	res := []*svcapitypes.Tag{}

	for {
		resp, err = rm.sdkapi.ListUserTagsWithContext(ctx, input)
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
	rm.metrics.RecordAPICall("GET", "ListUserTags", err)
	return res, err
}

// addTags adds the supplied Tags to the supplied User resource
func (rm *resourceManager) addTags(
	ctx context.Context,
	r *resource,
	tags []*svcapitypes.Tag,
) (err error) {
	rlog := ackrtlog.FromContext(ctx)
	exit := rlog.Trace("rm.addTag")
	defer func() {
		exit(err)
	}()

	input := &svcsdk.TagUserInput{}
	input.UserName = r.ko.Spec.Name
	inTags := []*svcsdk.Tag{}
	for _, t := range tags {
		inTags = append(inTags, &svcsdk.Tag{Key: t.Key, Value: t.Value})
	}
	input.Tags = inTags

	_, err = rm.sdkapi.TagUserWithContext(ctx, input)
	rm.metrics.RecordAPICall("UPDATE", "TagUser", err)
	return err
}

// removeTags removes the supplied Tags from the supplied User resource
func (rm *resourceManager) removeTags(
	ctx context.Context,
	r *resource,
	tags []*svcapitypes.Tag,
) (err error) {
	rlog := ackrtlog.FromContext(ctx)
	exit := rlog.Trace("rm.removeTag")
	defer func() {
		exit(err)
	}()

	input := &svcsdk.UntagUserInput{}
	input.UserName = r.ko.Spec.Name
	inTagKeys := []*string{}
	for _, t := range tags {
		inTagKeys = append(inTagKeys, t.Key)
	}
	input.TagKeys = inTagKeys

	_, err = rm.sdkapi.UntagUserWithContext(ctx, input)
	rm.metrics.RecordAPICall("UPDATE", "UntagUser", err)
	return err
}
