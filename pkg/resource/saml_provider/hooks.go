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

package saml_provider

import (
	"context"

	ackrtlog "github.com/aws-controllers-k8s/runtime/pkg/runtime/log"
	"github.com/aws/aws-sdk-go-v2/aws"
	svcsdk "github.com/aws/aws-sdk-go-v2/service/iam"
	svcsdktypes "github.com/aws/aws-sdk-go-v2/service/iam/types"

	svcapitypes "github.com/aws-controllers-k8s/iam-controller/apis/v1alpha1"
)

// getTags retrieves the tags for a given SAMLProvider
func (rm *resourceManager) getTags(
	ctx context.Context,
	arn string,
) []*svcapitypes.Tag {
	rlog := ackrtlog.FromContext(ctx)
	resp, err := rm.sdkapi.ListSAMLProviderTags(ctx, &svcsdk.ListSAMLProviderTagsInput{
		SAMLProviderArn: aws.String(arn),
	})
	if err != nil {
		rlog.Debug("error getting tags for SAMLProvider", "error", err)
		return nil
	}

	// Convert SDK tags to API tags
	apiTags := make([]*svcapitypes.Tag, len(resp.Tags))
	for i, tag := range resp.Tags {
		apiTags[i] = &svcapitypes.Tag{
			Key:   aws.String(*tag.Key),
			Value: aws.String(*tag.Value),
		}
	}

	return apiTags
}

// syncTags synchronizes tags between the desired and latest resource
func (rm *resourceManager) syncTags(
	ctx context.Context,
	latest *resource,
	desired *resource,
) error {
	rlog := ackrtlog.FromContext(ctx)
	latestTags := latest.ko.Spec.Tags
	desiredTags := desired.ko.Spec.Tags

	// If no tags are desired and there are no existing tags, we're done
	if len(desiredTags) == 0 && len(latestTags) == 0 {
		return nil
	}

	// Create maps for the latest and desired tags
	latestTagMap := make(map[string]string)
	for _, tag := range latestTags {
		latestTagMap[*tag.Key] = *tag.Value
	}

	desiredTagMap := make(map[string]string)
	for _, tag := range desiredTags {
		desiredTagMap[*tag.Key] = *tag.Value
	}

	// Find tags to remove (in latest but not in desired)
	var tagsToRemove []string
	for k := range latestTagMap {
		_, exists := desiredTagMap[k]
		if !exists {
			tagsToRemove = append(tagsToRemove, k)
		}
	}

	// Find tags to add or update (in desired but not in latest or with different values)
	tagsToAdd := make([]svcsdktypes.Tag, 0)
	for k, v := range desiredTagMap {
		latestVal, exists := latestTagMap[k]
		if !exists || latestVal != v {
			tagsToAdd = append(tagsToAdd, svcsdktypes.Tag{
				Key:   aws.String(k),
				Value: aws.String(v),
			})
		}
	}

	// Get the ARN from the resource
	arn := string(*latest.ko.Status.ACKResourceMetadata.ARN)

	// Remove tags if needed
	if len(tagsToRemove) > 0 {
		rlog.Debug("removing tags from SAMLProvider", "arn", arn, "tags", tagsToRemove)
		_, err := rm.sdkapi.UntagSAMLProvider(ctx, &svcsdk.UntagSAMLProviderInput{
			SAMLProviderArn: aws.String(arn),
			TagKeys:         tagsToRemove,
		})
		if err != nil {
			return err
		}
	}

	// Add tags if needed
	if len(tagsToAdd) > 0 {
		rlog.Debug("adding tags to SAMLProvider", "arn", arn, "tags", tagsToAdd)
		_, err := rm.sdkapi.TagSAMLProvider(ctx, &svcsdk.TagSAMLProviderInput{
			SAMLProviderArn: aws.String(arn),
			Tags:            tagsToAdd,
		})
		if err != nil {
			return err
		}
	}

	return nil
}
