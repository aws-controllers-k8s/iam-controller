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
	"net/url"

	ackcompare "github.com/aws-controllers-k8s/runtime/pkg/compare"
	ackrtlog "github.com/aws-controllers-k8s/runtime/pkg/runtime/log"
	ackutil "github.com/aws-controllers-k8s/runtime/pkg/util"
	svcsdk "github.com/aws/aws-sdk-go-v2/service/iam"
	svcsdktypes "github.com/aws/aws-sdk-go-v2/service/iam/types"
	"github.com/samber/lo"

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
	_, err = rm.sdkapi.PutUserPermissionsBoundary(ctx, input)
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
	_, err = rm.sdkapi.DeleteUserPermissionsBoundary(ctx, input)
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

// syncManagedPolicies examines the managed PolicyARNs in the supplied User and
// calls the ListUserPolicies, AttachUserPolicy and DetachUserPolicy APIs to
// ensure that the set of attached policies stays in sync with the
// User.Spec.Policies field, which is a list of strings containing Policy ARNs.
func (rm *resourceManager) syncManagedPolicies(
	ctx context.Context,
	desired *resource,
	latest *resource,
) (err error) {
	rlog := ackrtlog.FromContext(ctx)
	exit := rlog.Trace("rm.syncManagedPolicies")
	defer func() {
		exit(err)
	}()
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
		rlog.Debug("adding managed policy to user", "policy_arn", *p)
		if err = rm.addManagedPolicy(ctx, desired, p); err != nil {
			return err
		}
	}
	for _, p := range toDelete {
		rlog.Debug("removing managed policy from user", "policy_arn", *p)
		if err = rm.removeManagedPolicy(ctx, desired, p); err != nil {
			return err
		}
	}

	return nil
}

// getManagedPolicies returns the list of managed Policy ARNs currently
// attached to the User
func (rm *resourceManager) getManagedPolicies(
	ctx context.Context,
	r *resource,
) ([]*string, error) {
	var err error
	rlog := ackrtlog.FromContext(ctx)
	exit := rlog.Trace("rm.getManagedPolicies")
	defer func() {
		exit(err)
	}()

	input := &svcsdk.ListAttachedUserPoliciesInput{}
	input.UserName = r.ko.Spec.Name
	res := []*string{}

	paginator := svcsdk.NewListAttachedUserPoliciesPaginator(rm.sdkapi, input)
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		for _, p := range page.AttachedPolicies {
			res = append(res, p.PolicyArn)
		}
	}
	rm.metrics.RecordAPICall("READ_MANY", "ListAttachedUserPolicies", err)
	return res, err
}

// addManagedPolicy adds the supplied managed Policy to the supplied User
// resource
func (rm *resourceManager) addManagedPolicy(
	ctx context.Context,
	r *resource,
	policyARN *string,
) (err error) {
	rlog := ackrtlog.FromContext(ctx)
	exit := rlog.Trace("rm.addManagedPolicy")
	defer func() {
		exit(err)
	}()

	input := &svcsdk.AttachUserPolicyInput{}
	input.UserName = r.ko.Spec.Name
	input.PolicyArn = policyARN
	_, err = rm.sdkapi.AttachUserPolicy(ctx, input)
	rm.metrics.RecordAPICall("UPDATE", "AttachUserPolicy", err)
	return err
}

// removeManagedPolicy removes the supplied managed Policy from the supplied User resource
func (rm *resourceManager) removeManagedPolicy(
	ctx context.Context,
	r *resource,
	policyARN *string,
) (err error) {
	rlog := ackrtlog.FromContext(ctx)
	exit := rlog.Trace("rm.removeManagedPolicy")
	defer func() {
		exit(err)
	}()

	input := &svcsdk.DetachUserPolicyInput{}
	input.UserName = r.ko.Spec.Name
	input.PolicyArn = policyARN
	_, err = rm.sdkapi.DetachUserPolicy(ctx, input)
	rm.metrics.RecordAPICall("UPDATE", "DetachUserPolicy", err)
	return err
}

// syncInlinePolicies examines the InlinePolicies in the supplied User and
// calls the ListUserPolicies, PutUserPolicy and DeleteUserPolicy APIs to
// ensure that the set of attached policies stays in sync with the
// User.Spec.InlinePolicies field, which is a map of policy names to policy
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
			"adding inline policy to user",
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
			"removing inline policy from user",
			"policy_name", polName,
		)
		if err = rm.removeInlinePolicy(ctx, desired, polName); err != nil {
			return err
		}
	}

	return nil
}

// getInlinePolicies returns a map of inline policy name and policy docs
// currently attached to the User.
//
// NOTE(jaypipes): There's no way around the inefficiencies of this method
// without caching stuff, and I don't think it's useful to have an unbounded
// cache for these inline policy documents :( IAM's ListUserPolicies API call
// only returns the *policy names* of inline policies. You need to call
// GetUserPolicy API call for each inline policy name in order to get the
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

	userName := r.ko.Spec.Name

	input := &svcsdk.ListUserPoliciesInput{}
	input.UserName = userName
	res := map[string]*string{}

	paginator := svcsdk.NewListUserPoliciesPaginator(rm.sdkapi, input)
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		for _, p := range page.PolicyNames {
			res[p] = nil // Note: p is now a string, not *string
		}
	}
	rm.metrics.RecordAPICall("READ_MANY", "ListUserPolicies", err)

	// Now we need to grab the policy documents for each policy name
	for polName, _ := range res {
		input := &svcsdk.GetUserPolicyInput{}
		input.UserName = userName
		input.PolicyName = &polName
		resp, err := rm.sdkapi.GetUserPolicy(ctx, input)
		rm.metrics.RecordAPICall("READ_ONE", "GetUserPolicy", err)
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

// addInlinePolicy adds the supplied inline Policy to the supplied User
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

	input := &svcsdk.PutUserPolicyInput{}
	input.UserName = r.ko.Spec.Name
	input.PolicyName = &policyName
	cleanedDoc, err := decodeDocument(*policyDoc)
	if err != nil {
		return err
	}
	input.PolicyDocument = &cleanedDoc
	_, err = rm.sdkapi.PutUserPolicy(ctx, input)
	rm.metrics.RecordAPICall("UPDATE", "PutUserPolicy", err)
	return err
}

// removeInlinePolicy removes the supplied inline Policy from the supplied User
// resource
func (rm *resourceManager) removeInlinePolicy(
	ctx context.Context,
	r *resource,
	policyName string,
) (err error) {
	rlog := ackrtlog.FromContext(ctx)
	exit := rlog.Trace("rm.removeInlinePolicy")
	defer func() { exit(err) }()

	input := &svcsdk.DeleteUserPolicyInput{}
	input.UserName = r.ko.Spec.Name
	input.PolicyName = &policyName
	_, err = rm.sdkapi.DeleteUserPolicy(ctx, input)
	rm.metrics.RecordAPICall("UPDATE", "DeleteUserPolicy", err)
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
	desired *resource,
	latest *resource,
) (err error) {
	rlog := ackrtlog.FromContext(ctx)
	exit := rlog.Trace("rm.syncTags")
	defer func() {
		exit(err)
	}()
	toAdd := []*svcapitypes.Tag{}
	toDelete := []*svcapitypes.Tag{}

	existingTags := latest.ko.Spec.Tags

	for _, t := range desired.ko.Spec.Tags {
		if !inTags(*t.Key, *t.Value, existingTags) {
			toAdd = append(toAdd, t)
		}
	}

	for _, t := range existingTags {
		if !inTags(*t.Key, *t.Value, desired.ko.Spec.Tags) {
			toDelete = append(toDelete, t)
		}
	}

	if len(toAdd) > 0 {
		for _, t := range toAdd {
			rlog.Debug("adding tag to user", "key", *t.Key, "value", *t.Value)
		}
		if err = rm.addTags(ctx, desired, toAdd); err != nil {
			return err
		}
	}
	if len(toDelete) > 0 {
		for _, t := range toDelete {
			rlog.Debug("removing tag from user", "key", *t.Key, "value", *t.Value)
		}
		if err = rm.removeTags(ctx, desired, toDelete); err != nil {
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
		resp, err = rm.sdkapi.ListUserTags(ctx, input)
		if err != nil || resp == nil {
			break
		}
		for _, t := range resp.Tags {
			res = append(res, &svcapitypes.Tag{Key: t.Key, Value: t.Value})
		}
		if !resp.IsTruncated {
			break
		}
	}
	rm.metrics.RecordAPICall("READ_MANY", "ListUserTags", err)
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
	defer func() { exit(err) }()

	input := &svcsdk.TagUserInput{}
	input.UserName = r.ko.Spec.Name
	inTags := []svcsdktypes.Tag{}
	for _, t := range tags {
		inTags = append(inTags, svcsdktypes.Tag{Key: t.Key, Value: t.Value})
	}
	input.Tags = inTags

	_, err = rm.sdkapi.TagUser(ctx, input)
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
	inTagKeys := []string{}
	for _, t := range tags {
		inTagKeys = append(inTagKeys, *t.Key)
	}
	input.TagKeys = inTagKeys

	_, err = rm.sdkapi.UntagUser(ctx, input)
	rm.metrics.RecordAPICall("UPDATE", "UntagUser", err)
	return err
}

func decodeDocument(encoded string) (string, error) {
	return url.QueryUnescape(encoded)
}
