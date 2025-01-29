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

package open_id_connect_provider

import (
	"context"
	"fmt"
	"strings"

	ackcompare "github.com/aws-controllers-k8s/runtime/pkg/compare"
	ackcondition "github.com/aws-controllers-k8s/runtime/pkg/condition"
	ackerr "github.com/aws-controllers-k8s/runtime/pkg/errors"
	ackrtlog "github.com/aws-controllers-k8s/runtime/pkg/runtime/log"
	svcsdk "github.com/aws/aws-sdk-go-v2/service/iam"
	svcsdktypes "github.com/aws/aws-sdk-go-v2/service/iam/types"
	corev1 "k8s.io/api/core/v1"

	svcapitypes "github.com/aws-controllers-k8s/iam-controller/apis/v1alpha1"
	commonutil "github.com/aws-controllers-k8s/iam-controller/pkg/util"
)

func (rm *resourceManager) customUpdateOpenIDConnectProvider(
	ctx context.Context,
	desired *resource,
	latest *resource,
	delta *ackcompare.Delta,
) (updated *resource, err error) {
	rlog := ackrtlog.FromContext(ctx)
	exit := rlog.Trace("rm.customUpdateOpenIDConnectProvider")
	defer func() { exit(err) }()

	if immutableFieldChanges := rm.getImmutableFieldChanges(delta); len(immutableFieldChanges) > 0 {
		msg := fmt.Sprintf("Immutable Spec fields have been modified: %s", strings.Join(immutableFieldChanges, ","))
		return nil, ackerr.NewTerminalError(fmt.Errorf(msg))
	}

	if delta.DifferentAt("Spec.Thumbprints") {
		// Update the thumbprint list
		thumbprintInput, err := rm.newUpdateThumbprintRequestPayload(ctx, desired)
		if err != nil {
			return nil, err
		}

		var thumbprintResp *svcsdk.UpdateOpenIDConnectProviderThumbprintOutput
		_ = thumbprintResp
		thumbprintResp, err = rm.sdkapi.UpdateOpenIDConnectProviderThumbprint(ctx, thumbprintInput)
		rm.metrics.RecordAPICall("UPDATE", "UpdateOpenIDConnectProviderThumbprint", err)
		if err != nil {
			return nil, err
		}
	}

	if delta.DifferentAt("Spec.ClientIDs") {
		// Update the client ID list
		// here we only have an "add" and a "remove"
		// https://docs.aws.amazon.com/sdk-for-go/api/service/iam/#IAM.AddClientIDToOpenIDConnectProvider and
		// https://docs.aws.amazon.com/sdk-for-go/api/service/iam/#IAM.RemoveClientIDFromOpenIDConnectProvider
		// so we have to compute the diff ourselves
		desiredClientIDs := map[string]bool{}
		latestClientIDs := map[string]bool{}
		for _, desiredClientID := range desired.ko.Spec.ClientIDs {
			desiredClientIDs[*desiredClientID] = true
		}
		for _, latestClientID := range latest.ko.Spec.ClientIDs {
			latestClientIDs[*latestClientID] = true
		}

		for desiredClientID, _ := range desiredClientIDs {
			_, hasLatest := latestClientIDs[desiredClientID]
			if !hasLatest {
				// clientID is to be added

				addClientIDInput, err := rm.newAddClientIDRequestPayload(ctx, desired, &desiredClientID)
				if err != nil {
					return nil, err
				}

				var addClientIDResp *svcsdk.AddClientIDToOpenIDConnectProviderOutput
				_ = addClientIDResp
				addClientIDResp, err = rm.sdkapi.AddClientIDToOpenIDConnectProvider(ctx, addClientIDInput)
				rm.metrics.RecordAPICall("UPDATE", "AddClientIDToOpenIDConnectProvider", err)
				if err != nil {
					return nil, err
				}
			} else {
				delete(desiredClientIDs, desiredClientID)
				delete(latestClientIDs, desiredClientID)
			}
		}
		for latestClientID, _ := range latestClientIDs {
			// clientID is to be removed
			removeClientIDInput, err := rm.newRemoveClientIDRequestPayload(ctx, desired, &latestClientID)
			if err != nil {
				return nil, err
			}

			var removeClientIDResp *svcsdk.RemoveClientIDFromOpenIDConnectProviderOutput
			_ = removeClientIDResp
			removeClientIDResp, err = rm.sdkapi.RemoveClientIDFromOpenIDConnectProvider(ctx, removeClientIDInput)
			rm.metrics.RecordAPICall("UPDATE", "RemoveClientIDFromOpenIDConnectProvider", err)
			if err != nil {
				return nil, err
			}
		}
	}

	if delta.DifferentAt("Spec.Tags") {
		if err := rm.syncTags(ctx, desired); err != nil {
			return nil, err
		}
	}
	// There really isn't a status of a role... it either exists or doesn't. If
	// we get here, that means the update was successful and the desired state
	// of the role matches what we provided...
	ackcondition.SetSynced(desired, corev1.ConditionTrue, nil, nil)

	return desired, nil
}

// custom comparison function for comparing
//   - lists of Tag structs where the order of the structs in the list is not important.
//   - URLs where a prefix of https:// should be disregarded
func customPreCompare(
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

	if ackcompare.HasNilDifference(a.ko.Spec.URL, b.ko.Spec.URL) {
		delta.Add("Spec.URL", a.ko.Spec.URL, b.ko.Spec.URL)
	} else if a.ko.Spec.URL != nil && b.ko.Spec.URL != nil {
		// the URL field must begin with "https://"
		// c.f. https://docs.aws.amazon.com/IAM/latest/APIReference/API_CreateOpenIDConnectProvider.html
		// however, when stored in the IAM backend, the "https://" prefix is stripped out
		// thus here we treat "https://" as a null token for the purposes of string comparison
		if *a.ko.Spec.URL != *b.ko.Spec.URL {
			aURL := strings.TrimPrefix(*a.ko.Spec.URL, "https://")
			bURL := strings.TrimPrefix(*b.ko.Spec.URL, "https://")
			if aURL != bURL {
				delta.Add("Spec.URL", a.ko.Spec.URL, b.ko.Spec.URL)
			}
		}
	}
}

// syncTags examines the Tags in the supplied OpenIDConnectProvider and calls the ListOpenIDConnectProviderTags,
// TagOpenIDConnectProvider and UntagOpenIDConnectProvider API endpoints to ensure that the set of associated Tags stays
// in sync with the OpenIDConnectProvider.Spec.Tags
func (rm *resourceManager) syncTags(
	ctx context.Context,
	r *resource,
) (err error) {
	rlog := ackrtlog.FromContext(ctx)
	exit := rlog.Trace("rm.syncTags")
	defer func(err error) { exit(err) }(err)
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
			rlog.Debug("adding tag to OpenIDConnectProvider", "key", *t.Key, "value", *t.Value)
		}
		if err = rm.addTags(ctx, r, toAdd); err != nil {
			return err
		}
	}
	if len(toDelete) > 0 {
		for _, t := range toDelete {
			rlog.Debug("removing tag from OpenIDConnectProvider", "key", *t.Key, "value", *t.Value)
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
		if *t.Key == key && t.Value != nil && *t.Value == value {
			return true
		}
	}
	return false
}

// getTags returns the list of tags attached to the OpenIDConnectProvider
func (rm *resourceManager) getTags(
	ctx context.Context,
	r *resource,
) ([]*svcapitypes.Tag, error) {
	var err error
	var resp *svcsdk.ListOpenIDConnectProviderTagsOutput
	rlog := ackrtlog.FromContext(ctx)
	exit := rlog.Trace("rm.getTags")
	defer func() { exit(err) }()

	input := &svcsdk.ListOpenIDConnectProviderTagsInput{}
	input.OpenIDConnectProviderArn = (*string)(r.ko.Status.ACKResourceMetadata.ARN)
	res := []*svcapitypes.Tag{}

	for {
		resp, err = rm.sdkapi.ListOpenIDConnectProviderTags(ctx, input)
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
		rm.metrics.RecordAPICall("READ_MANY", "ListOpenIDConnectProviderTags", err)
	}
	return res, err
}

// addTags adds the supplied Tags to the supplied OpenIDConnectProvider resource
func (rm *resourceManager) addTags(
	ctx context.Context,
	r *resource,
	tags []*svcapitypes.Tag,
) (err error) {
	rlog := ackrtlog.FromContext(ctx)
	exit := rlog.Trace("rm.addTags")
	defer func() { exit(err) }()

	input := &svcsdk.TagOpenIDConnectProviderInput{}
	input.OpenIDConnectProviderArn = (*string)(r.ko.Status.ACKResourceMetadata.ARN)
	inTags := []svcsdktypes.Tag{}
	for _, t := range tags {
		inTags = append(inTags, svcsdktypes.Tag{Key: t.Key, Value: t.Value})
	}
	input.Tags = inTags

	_, err = rm.sdkapi.TagOpenIDConnectProvider(ctx, input)
	rm.metrics.RecordAPICall("UPDATE", "TagOpenIDConnectProvider", err)
	return err
}

// removeTags removes the supplied Tags from the supplied OpenIDConnectProvider resource
func (rm *resourceManager) removeTags(
	ctx context.Context,
	r *resource,
	tags []*svcapitypes.Tag,
) (err error) {
	rlog := ackrtlog.FromContext(ctx)
	exit := rlog.Trace("rm.removeTags")
	defer func() { exit(err) }()

	input := &svcsdk.UntagOpenIDConnectProviderInput{}
	input.OpenIDConnectProviderArn = (*string)(r.ko.Status.ACKResourceMetadata.ARN)
	inTagKeys := []string{}
	for _, t := range tags {
		inTagKeys = append(inTagKeys, *t.Key)
	}
	input.TagKeys = inTagKeys

	_, err = rm.sdkapi.UntagOpenIDConnectProvider(ctx, input)
	rm.metrics.RecordAPICall("UPDATE", "UntagOpenIDConnectProvider", err)
	return err
}

// returns an SDK-specific struct for the HTTP request
// payload of the UpdateThumbprint API call for the resource
func (rm *resourceManager) newUpdateThumbprintRequestPayload(
	ctx context.Context,
	r *resource,
) (*svcsdk.UpdateOpenIDConnectProviderThumbprintInput, error) {
	res := &svcsdk.UpdateOpenIDConnectProviderThumbprintInput{}

	if r.ko.Spec.Thumbprints != nil {
		thumbprints := []string{}
		for _, thumbprint := range r.ko.Spec.Thumbprints {
			thumbprints = append(thumbprints, *thumbprint)
		}
		res.ThumbprintList = thumbprints
	}
	if r.ko.Status.ACKResourceMetadata.ARN != nil {
		arnStr := string(*r.ko.Status.ACKResourceMetadata.ARN)
		res.OpenIDConnectProviderArn = &arnStr
	}

	return res, nil
}

// returns an SDK-specific struct for the HTTP request
// payload of the AddClientIDToOpenIDConnectProvider API call for the resource
func (rm *resourceManager) newAddClientIDRequestPayload(
	ctx context.Context,
	r *resource,
	clientId *string,
) (*svcsdk.AddClientIDToOpenIDConnectProviderInput, error) {
	res := &svcsdk.AddClientIDToOpenIDConnectProviderInput{}

	if clientId != nil {
		res.ClientID = clientId
	}
	if r.ko.Status.ACKResourceMetadata.ARN != nil {
		arnStr := string(*r.ko.Status.ACKResourceMetadata.ARN)
		res.OpenIDConnectProviderArn = &arnStr
	}

	return res, nil
}

// returns an SDK-specific struct for the HTTP request
// payload of the RemoveClientIDFromOpenIDConnectProvider API call for the resource
func (rm *resourceManager) newRemoveClientIDRequestPayload(
	ctx context.Context,
	r *resource,
	clientId *string,
) (*svcsdk.RemoveClientIDFromOpenIDConnectProviderInput, error) {
	res := &svcsdk.RemoveClientIDFromOpenIDConnectProviderInput{}

	if clientId != nil {
		res.ClientID = clientId
	}
	if r.ko.Status.ACKResourceMetadata.ARN != nil {
		arnStr := string(*r.ko.Status.ACKResourceMetadata.ARN)
		res.OpenIDConnectProviderArn = &arnStr
	}

	return res, nil
}
