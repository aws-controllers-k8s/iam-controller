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

package tags

import (
	"context"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/iam"
	"github.com/aws/aws-sdk-go/service/iam/iamiface"
)

// TagsSAMLProvider returns the tags for a given IAM SAML Provider
func TagsSAMLProvider(
	ctx context.Context,
	api iamiface.IAMAPI,
	arn string,
) ([]*iam.Tag, error) {
	resp, err := api.ListSAMLProviderTagsWithContext(
		ctx,
		&iam.ListSAMLProviderTagsInput{
			SAMLProviderArn: aws.String(arn),
		},
	)
	if err != nil {
		return nil, err
	}
	return resp.Tags, nil
}

// SyncTagsSAMLProvider syncs the tags for an IAM SAML Provider by adding and removing tags
func SyncTagsSAMLProvider(
	ctx context.Context,
	api iamiface.IAMAPI,
	arn string,
	desired []*iam.Tag,
	latest []*iam.Tag,
) (err error) {
	// Create maps for the latest and desired tags
	latestMap := make(map[string]string)
	for _, tag := range latest {
		latestMap[*tag.Key] = *tag.Value
	}

	desiredMap := make(map[string]string)
	for _, tag := range desired {
		desiredMap[*tag.Key] = *tag.Value
	}

	// Find tags to remove (in latest but not in desired or different value)
	var tagsToRemove []string
	for k := range latestMap {
		_, exists := desiredMap[k]
		if !exists {
			tagsToRemove = append(tagsToRemove, k)
		}
	}

	// Find tags to add (in desired but not in latest or different value)
	tagsToAdd := make(map[string]string)
	for k, v := range desiredMap {
		latestVal, exists := latestMap[k]
		if !exists || latestVal != v {
			tagsToAdd[k] = v
		}
	}

	if len(tagsToRemove) > 0 {
		_, err = api.UntagSAMLProviderWithContext(
			ctx,
			&iam.UntagSAMLProviderInput{
				SAMLProviderArn: aws.String(arn),
				TagKeys:         aws.StringSlice(tagsToRemove),
			},
		)
		if err != nil {
			return err
		}
	}

	if len(tagsToAdd) > 0 {
		// Convert the map of tags to add into a slice of IAM Tag pointers
		tags := make([]*iam.Tag, 0, len(tagsToAdd))
		for k, v := range tagsToAdd {
			tags = append(tags, &iam.Tag{
				Key:   aws.String(k),
				Value: aws.String(v),
			})
		}
		_, err = api.TagSAMLProviderWithContext(
			ctx,
			&iam.TagSAMLProviderInput{
				SAMLProviderArn: aws.String(arn),
				Tags:            tags,
			},
		)
		if err != nil {
			return err
		}
	}

	return nil
}

// CompareTags compares two sets of tags and returns true if they are equal
func CompareTags(
	a []*iam.Tag,
	b []*iam.Tag,
) bool {
	if len(a) != len(b) {
		return false
	}

	aMap := make(map[string]string, len(a))
	for _, tag := range a {
		aMap[*tag.Key] = *tag.Value
	}

	for _, tag := range b {
		if val, ok := aMap[*tag.Key]; !ok || val != *tag.Value {
			return false
		}
	}

	return true
}
