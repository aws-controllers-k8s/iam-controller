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
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/service/iam"
	"github.com/aws/aws-sdk-go/service/iam/iamiface"
	"github.com/stretchr/testify/assert"
)

// mockIAMClient is a fake implementation of the IAM API interface
type mockIAMClient struct {
	iamiface.IAMAPI
	// Track calls to the API methods
	listTagsCalls      int
	tagResourceCalls   int
	untagResourceCalls int
	// Mock responses
	listTagsOutput *iam.ListSAMLProviderTagsOutput
	tagOutput      *iam.TagSAMLProviderOutput
	untagOutput    *iam.UntagSAMLProviderOutput
	// Capture inputs for verification
	tagInput   *iam.TagSAMLProviderInput
	untagInput *iam.UntagSAMLProviderInput
}

func (m *mockIAMClient) ListSAMLProviderTagsWithContext(
	_ context.Context,
	input *iam.ListSAMLProviderTagsInput,
	_ ...request.Option,
) (*iam.ListSAMLProviderTagsOutput, error) {
	m.listTagsCalls++
	return m.listTagsOutput, nil
}

func (m *mockIAMClient) TagSAMLProviderWithContext(
	_ context.Context,
	input *iam.TagSAMLProviderInput,
	_ ...request.Option,
) (*iam.TagSAMLProviderOutput, error) {
	m.tagResourceCalls++
	m.tagInput = input
	return m.tagOutput, nil
}

func (m *mockIAMClient) UntagSAMLProviderWithContext(
	_ context.Context,
	input *iam.UntagSAMLProviderInput,
	_ ...request.Option,
) (*iam.UntagSAMLProviderOutput, error) {
	m.untagResourceCalls++
	m.untagInput = input
	return m.untagOutput, nil
}

func TestTagsSAMLProvider(t *testing.T) {
	ctx := context.Background()
	mockClient := &mockIAMClient{
		listTagsOutput: &iam.ListSAMLProviderTagsOutput{
			Tags: []*iam.Tag{
				{
					Key:   aws.String("key1"),
					Value: aws.String("value1"),
				},
				{
					Key:   aws.String("key2"),
					Value: aws.String("value2"),
				},
			},
		},
	}

	tags, err := TagsSAMLProvider(ctx, mockClient, "arn:aws:iam::123456789012:saml-provider/test")

	assert.NoError(t, err)
	assert.Equal(t, 1, mockClient.listTagsCalls)
	assert.Len(t, tags, 2)
	assert.Equal(t, "key1", *tags[0].Key)
	assert.Equal(t, "value1", *tags[0].Value)
	assert.Equal(t, "key2", *tags[1].Key)
	assert.Equal(t, "value2", *tags[1].Value)
}

func TestSyncTagsSAMLProvider(t *testing.T) {
	ctx := context.Background()
	mockClient := &mockIAMClient{
		tagOutput:   &iam.TagSAMLProviderOutput{},
		untagOutput: &iam.UntagSAMLProviderOutput{},
	}

	testCases := []struct {
		name            string
		desired         []*iam.Tag
		latest          []*iam.Tag
		expectTagCall   bool
		expectUntagCall bool
	}{
		{
			name: "No changes",
			desired: []*iam.Tag{
				{
					Key:   aws.String("key1"),
					Value: aws.String("value1"),
				},
			},
			latest: []*iam.Tag{
				{
					Key:   aws.String("key1"),
					Value: aws.String("value1"),
				},
			},
			expectTagCall:   false,
			expectUntagCall: false,
		},
		{
			name: "Add tag",
			desired: []*iam.Tag{
				{
					Key:   aws.String("key1"),
					Value: aws.String("value1"),
				},
				{
					Key:   aws.String("key2"),
					Value: aws.String("value2"),
				},
			},
			latest: []*iam.Tag{
				{
					Key:   aws.String("key1"),
					Value: aws.String("value1"),
				},
			},
			expectTagCall:   true,
			expectUntagCall: false,
		},
		{
			name: "Remove tag",
			desired: []*iam.Tag{
				{
					Key:   aws.String("key1"),
					Value: aws.String("value1"),
				},
			},
			latest: []*iam.Tag{
				{
					Key:   aws.String("key1"),
					Value: aws.String("value1"),
				},
				{
					Key:   aws.String("key2"),
					Value: aws.String("value2"),
				},
			},
			expectTagCall:   false,
			expectUntagCall: true,
		},
		{
			name: "Update tag value",
			desired: []*iam.Tag{
				{
					Key:   aws.String("key1"),
					Value: aws.String("newvalue1"),
				},
			},
			latest: []*iam.Tag{
				{
					Key:   aws.String("key1"),
					Value: aws.String("value1"),
				},
			},
			expectTagCall:   true,
			expectUntagCall: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Reset mock counters
			mockClient.tagResourceCalls = 0
			mockClient.untagResourceCalls = 0
			mockClient.tagInput = nil
			mockClient.untagInput = nil

			err := SyncTagsSAMLProvider(ctx, mockClient, "arn:aws:iam::123456789012:saml-provider/test", tc.desired, tc.latest)

			assert.NoError(t, err)
			if tc.expectTagCall {
				assert.Equal(t, 1, mockClient.tagResourceCalls)
				assert.NotNil(t, mockClient.tagInput)
			} else {
				assert.Equal(t, 0, mockClient.tagResourceCalls)
			}

			if tc.expectUntagCall {
				assert.Equal(t, 1, mockClient.untagResourceCalls)
				assert.NotNil(t, mockClient.untagInput)
			} else {
				assert.Equal(t, 0, mockClient.untagResourceCalls)
			}
		})
	}
}

func TestCompareTags(t *testing.T) {
	testCases := []struct {
		name     string
		a        []*iam.Tag
		b        []*iam.Tag
		expected bool
	}{
		{
			name: "Equal tags",
			a: []*iam.Tag{
				{
					Key:   aws.String("key1"),
					Value: aws.String("value1"),
				},
				{
					Key:   aws.String("key2"),
					Value: aws.String("value2"),
				},
			},
			b: []*iam.Tag{
				{
					Key:   aws.String("key1"),
					Value: aws.String("value1"),
				},
				{
					Key:   aws.String("key2"),
					Value: aws.String("value2"),
				},
			},
			expected: true,
		},
		{
			name: "Different values",
			a: []*iam.Tag{
				{
					Key:   aws.String("key1"),
					Value: aws.String("value1"),
				},
			},
			b: []*iam.Tag{
				{
					Key:   aws.String("key1"),
					Value: aws.String("different"),
				},
			},
			expected: false,
		},
		{
			name: "Different lengths",
			a: []*iam.Tag{
				{
					Key:   aws.String("key1"),
					Value: aws.String("value1"),
				},
				{
					Key:   aws.String("key2"),
					Value: aws.String("value2"),
				},
			},
			b: []*iam.Tag{
				{
					Key:   aws.String("key1"),
					Value: aws.String("value1"),
				},
			},
			expected: false,
		},
		{
			name: "Different keys",
			a: []*iam.Tag{
				{
					Key:   aws.String("key1"),
					Value: aws.String("value1"),
				},
			},
			b: []*iam.Tag{
				{
					Key:   aws.String("different"),
					Value: aws.String("value1"),
				},
			},
			expected: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := CompareTags(tc.a, tc.b)
			assert.Equal(t, tc.expected, result)
		})
	}
}
