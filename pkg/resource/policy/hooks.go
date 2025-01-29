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

package policy

import (
	"context"
	"net/url"
	"sort"
	"time"

	ackcompare "github.com/aws-controllers-k8s/runtime/pkg/compare"
	ackcondition "github.com/aws-controllers-k8s/runtime/pkg/condition"
	ackrtlog "github.com/aws-controllers-k8s/runtime/pkg/runtime/log"
	svcsdk "github.com/aws/aws-sdk-go-v2/service/iam"
	svcsdktypes "github.com/aws/aws-sdk-go-v2/service/iam/types"
	corev1 "k8s.io/api/core/v1"

	svcapitypes "github.com/aws-controllers-k8s/iam-controller/apis/v1alpha1"
	commonutil "github.com/aws-controllers-k8s/iam-controller/pkg/util"
)

const (
	// limitPolicyVersions is the max allowed number of policy versions you can
	// have before needing to delete a policy version.
	//
	// https://docs.aws.amazon.com/IAM/latest/APIReference/API_CreatePolicyVersion.html
	limitPolicyVersions = 5
)

func (rm *resourceManager) customUpdatePolicy(
	ctx context.Context,
	desired *resource,
	latest *resource,
	delta *ackcompare.Delta,
) (*resource, error) {
	ko := desired.ko.DeepCopy()

	rm.setStatusDefaults(ko)

	if delta.DifferentAt("Spec.Tags") {
		if err := rm.syncTags(ctx, desired, latest); err != nil {
			return nil, err
		}
	}

	if delta.DifferentAt("Spec.PolicyDocument") {
		newVersionID, err := rm.updatePolicyDocument(ctx, desired)
		if err != nil {
			return nil, err
		}
		ko.Status.DefaultVersionID = &newVersionID
	}

	// There really isn't a status of a policy... it either exists or doesn't.
	// If we get here, that means the update was successful and the desired
	// state of the policy matches what we provided...
	ackcondition.SetSynced(&resource{ko}, corev1.ConditionTrue, nil, nil)

	return &resource{ko}, nil
}

// syncTags examines the Tags in the supplied Policy and calls the
// ListPolicyTags, TagPolicy and UntagPolicy APIs to ensure that the set of
// associated Tags  stays in sync with the Policy.Spec.Tags
func (rm *resourceManager) syncTags(
	ctx context.Context,
	desired *resource,
	latest *resource,
) (err error) {
	rlog := ackrtlog.FromContext(ctx)
	exit := rlog.Trace("rm.syncTags")
	defer func() { exit(err) }()
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

	if len(toDelete) > 0 {
		for _, t := range toDelete {
			rlog.Debug("removing tag from policy", "key", *t.Key, "value", *t.Value)
		}
		if err = rm.removeTags(ctx, desired, toDelete); err != nil {
			return err
		}
	}
	if len(toAdd) > 0 {
		for _, t := range toAdd {
			rlog.Debug("adding tag to policy", "key", *t.Key, "value", *t.Value)
		}
		if err = rm.addTags(ctx, desired, toAdd); err != nil {
			return err
		}
	}

	return nil
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

// getTags returns the list of tags attached to the Policy
func (rm *resourceManager) getTags(
	ctx context.Context,
	r *resource,
) ([]*svcapitypes.Tag, error) {
	var err error
	var resp *svcsdk.ListPolicyTagsOutput
	rlog := ackrtlog.FromContext(ctx)
	exit := rlog.Trace("rm.getTags")
	defer func() { exit(err) }()

	input := &svcsdk.ListPolicyTagsInput{}
	input.PolicyArn = (*string)(r.ko.Status.ACKResourceMetadata.ARN)
	res := []*svcapitypes.Tag{}

	for {
		resp, err = rm.sdkapi.ListPolicyTags(ctx, input)
		if err != nil || resp == nil {
			break
		}
		for _, t := range resp.Tags {
			res = append(res, &svcapitypes.Tag{Key: t.Key, Value: t.Value})
		}
		if !resp.IsTruncated {
			break
		}
		input.Marker = resp.Marker
		rm.metrics.RecordAPICall("READ_MANY", "ListPolicyTags", err)
	}
	return res, err
}

// addTags adds the supplied Tags to the supplied Policy resource
func (rm *resourceManager) addTags(
	ctx context.Context,
	r *resource,
	tags []*svcapitypes.Tag,
) (err error) {
	rlog := ackrtlog.FromContext(ctx)
	exit := rlog.Trace("rm.addTag")
	defer func() { exit(err) }()

	input := &svcsdk.TagPolicyInput{}
	input.PolicyArn = (*string)(r.ko.Status.ACKResourceMetadata.ARN)
	inTags := []svcsdktypes.Tag{}
	for _, t := range tags {
		inTags = append(inTags, svcsdktypes.Tag{Key: t.Key, Value: t.Value})
	}
	input.Tags = inTags

	_, err = rm.sdkapi.TagPolicy(ctx, input)
	rm.metrics.RecordAPICall("UPDATE", "TagPolicy", err)
	return err
}

// removeTags removes the supplied Tags from the supplied Policy resource
func (rm *resourceManager) removeTags(
	ctx context.Context,
	r *resource,
	tags []*svcapitypes.Tag,
) (err error) {
	rlog := ackrtlog.FromContext(ctx)
	exit := rlog.Trace("rm.removeTag")
	defer func() { exit(err) }()

	input := &svcsdk.UntagPolicyInput{}
	input.PolicyArn = (*string)(r.ko.Status.ACKResourceMetadata.ARN)
	inTagKeys := []string{}
	for _, t := range tags {
		inTagKeys = append(inTagKeys, *t.Key)
	}
	input.TagKeys = inTagKeys

	_, err = rm.sdkapi.UntagPolicy(ctx, input)
	rm.metrics.RecordAPICall("UPDATE", "UntagPolicy", err)
	return err
}

// updatePolicyDocument creates a new Policy version with the new
// PolicyDocument and returns the newly-created version ID.
//
// A policy is technically immutable. In order to modify the PolicyDocument,
// one calls the CreatePolicyVersion API call and creates a new Policy version
// with the updated PolicyDocument.
func (rm *resourceManager) updatePolicyDocument(
	ctx context.Context,
	r *resource,
) (string, error) {
	var err error
	rlog := ackrtlog.FromContext(ctx)
	exit := rlog.Trace("rm.updatePolicyDocument")
	defer func() { exit(err) }()

	policyARN := (*string)(r.ko.Status.ACKResourceMetadata.ARN)

	if err = rm.ensureVersionsLimitNotExceeded(ctx, *policyARN); err != nil {
		return "", err
	}

	input := &svcsdk.CreatePolicyVersionInput{}
	input.PolicyArn = policyARN
	input.PolicyDocument = r.ko.Spec.PolicyDocument

	input.SetAsDefault = true

	resp, err := rm.sdkapi.CreatePolicyVersion(ctx, input)
	rm.metrics.RecordAPICall("UPDATE", "CreatePolicyVersion", err)
	if err != nil {
		return "", err
	}
	return *resp.PolicyVersion.VersionId, err
}

// ensureVersionsLimitNotExceeded checks to see if the number of versions
// for a supplied managed policy ARN exceeds 4 and deletes the oldest policy
// version if so.
//
// According to the IAM docs:
//
// > A managed policy can have up to five versions. If the policy has five
// > versions, you must delete an existing version using DeletePolicyVersion
// > before you create a new version.
//
// https://docs.aws.amazon.com/IAM/latest/APIReference/API_CreatePolicyVersion.html
func (rm *resourceManager) ensureVersionsLimitNotExceeded(
	ctx context.Context,
	policyARN string,
) (err error) {
	rlog := ackrtlog.FromContext(ctx)
	exit := rlog.Trace("rm.ensureVersionsLimitNotExceeded")
	defer func() { exit(err) }()

	versions, err := rm.getPolicyVersions(ctx, policyARN)
	if err != nil {
		return err
	}

	if len(versions) == limitPolicyVersions {
		for _, v := range versions {
			if v.isDefault {
				continue
			}
			err = rm.deletePolicyVersion(ctx, policyARN, v.version)
			if err != nil {
				return err
			}
			rlog.Info(
				"exceeded limit of policy versions. deleted earliest policy "+
					"version (non-default) before adding new version.",
				"policy_version", v.version,
			)
			break
		}
	}

	return nil
}

type policyVersion struct {
	version    string
	createDate *time.Time
	document   string
	isDefault  bool
}

// getPolicyVersion gets the specified policy version from the supplied policy
func (rm *resourceManager) getPolicyVersion(
	ctx context.Context,
	policyARN string,
	version string,
) (pv *policyVersion, err error) {
	rlog := ackrtlog.FromContext(ctx)
	exit := rlog.Trace("rm.getPolicyVersion")
	defer func() { exit(err) }()

	input := &svcsdk.GetPolicyVersionInput{}
	input.PolicyArn = &policyARN
	input.VersionId = &version

	var resp *svcsdk.GetPolicyVersionOutput
	resp, err = rm.sdkapi.GetPolicyVersion(ctx, input)
	rm.metrics.RecordAPICall("READ_ONE", "GetPolicyVersion", err)

	if err != nil {
		return nil, err
	}
	pv = &policyVersion{}
	if resp.PolicyVersion != nil {
		pv.createDate = resp.PolicyVersion.CreateDate
		if resp.PolicyVersion.VersionId != nil {
			pv.version = *resp.PolicyVersion.VersionId
		}
		if resp.PolicyVersion.Document != nil {
			// The policy document is URL-encoded by default, which leads to
			// false positive deltas...
			rawDoc := *resp.PolicyVersion.Document
			doc, err := url.QueryUnescape(rawDoc)
			if err != nil {
				return nil, err
			}
			pv.document = doc
		}
		pv.isDefault = resp.PolicyVersion.IsDefaultVersion
	}
	return pv, nil
}

// getPolicyVersions returns a slice, sorted by creation date, of the policy
// versions for a supplied Policy ARN.
func (rm *resourceManager) getPolicyVersions(
	ctx context.Context,
	policyARN string,
) (versions []policyVersion, err error) {
	rlog := ackrtlog.FromContext(ctx)
	exit := rlog.Trace("rm.getPolicyVersions")
	defer func() { exit(err) }()

	input := &svcsdk.ListPolicyVersionsInput{}
	input.PolicyArn = &policyARN

	var resp *svcsdk.ListPolicyVersionsOutput
	versions = []policyVersion{}
	for {
		resp, err = rm.sdkapi.ListPolicyVersions(ctx, input)
		if err != nil || resp == nil {
			break
		}
		for _, v := range resp.Versions {
			// NOTE(jaypipes): Deliberately skipping the PolicyDocument because
			// we don't use this information in callers of this function. The
			// singular getPolicyVersion() method *does* return the
			// PolicyDocument, however, and that method is called to populate
			// the Spec.PolicyDocument in sdkFind()
			pv := policyVersion{
				version:    *v.VersionId,
				createDate: v.CreateDate,
				isDefault:  v.IsDefaultVersion,
			}
			versions = append(versions, pv)
		}
		if !resp.IsTruncated {
			break
		}
		input.Marker = resp.Marker
		rm.metrics.RecordAPICall("READ_MANY", "ListPolicyVersions", err)
	}

	// Sort the list of versions by the creation date and return the list of
	// versions in creation date order
	sort.Slice(versions, func(i, j int) bool {
		return versions[i].createDate.Before(*versions[j].createDate)
	})
	return versions, err
}

// deletePolicyVersion removes the specified policy version from the supplied
// policy
func (rm *resourceManager) deletePolicyVersion(
	ctx context.Context,
	policyARN string,
	version string,
) (err error) {
	rlog := ackrtlog.FromContext(ctx)
	exit := rlog.Trace("rm.deletePolicyVersion")
	defer func() { exit(err) }()

	input := &svcsdk.DeletePolicyVersionInput{}
	input.PolicyArn = &policyARN
	input.VersionId = &version

	_, err = rm.sdkapi.DeletePolicyVersion(ctx, input)
	rm.metrics.RecordAPICall("UPDATE", "DeletePolicyVersion", err)
	return err
}

// deleteNonDefaultPolicyVersions removes all policy versions other than the
// default version (which is deleted when the policy itself is deleted).
func (rm *resourceManager) deleteNonDefaultPolicyVersions(
	ctx context.Context,
	r *resource,
) (err error) {
	rlog := ackrtlog.FromContext(ctx)
	exit := rlog.Trace("rm.deleteNonDefaultPolicyVersions")
	defer func() { exit(err) }()

	policyARN := string(*r.ko.Status.ACKResourceMetadata.ARN)

	versions, err := rm.getPolicyVersions(ctx, policyARN)
	if err != nil {
		return err
	}

	for _, v := range versions {
		if v.isDefault {
			continue
		}
		pver := v.version
		if err = rm.deletePolicyVersion(ctx, policyARN, pver); err != nil {
			return err
		}
		rlog.Info(
			"deleted non-default policy version",
			"policy_arn", policyARN,
			"policy_version", pver,
		)
	}
	return nil
}
