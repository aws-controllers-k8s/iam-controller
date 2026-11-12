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
	"testing"

	ackv1alpha1 "github.com/aws-controllers-k8s/runtime/apis/core/v1alpha1"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	svcapitypes "github.com/aws-controllers-k8s/iam-controller/apis/v1alpha1"
)

func policyResource(annotations map[string]string, name, path *string) *resource {
	return &resource{
		ko: &svcapitypes.Policy{
			ObjectMeta: metav1.ObjectMeta{Annotations: annotations},
			Spec: svcapitypes.PolicySpec{
				Name: name,
				Path: path,
			},
		},
	}
}

func TestPopulateResourceFromAnnotation(t *testing.T) {
	tests := []struct {
		name     string
		fields   map[string]string
		wantErr  bool
		wantName *string
		wantPath *string
		wantARN  *string
	}{
		{
			name:    "arn",
			fields:  map[string]string{"arn": "arn:aws:iam::123456789012:policy/p"},
			wantARN: aws.String("arn:aws:iam::123456789012:policy/p"),
		},
		{
			name:    "arn takes precedence over name",
			fields:  map[string]string{"arn": "arn:aws:iam::123456789012:policy/p", "name": "other"},
			wantARN: aws.String("arn:aws:iam::123456789012:policy/p"),
		},
		{
			name:     "name",
			fields:   map[string]string{"name": "p"},
			wantName: aws.String("p"),
		},
		{
			name:     "name and path",
			fields:   map[string]string{"name": "p", "path": "/team/"},
			wantName: aws.String("p"),
			wantPath: aws.String("/team/"),
		},
		{
			name:    "neither arn nor name",
			fields:  map[string]string{"path": "/team/"},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := policyResource(nil, nil, nil)
			err := r.PopulateResourceFromAnnotation(tt.fields)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantName, r.ko.Spec.Name)
			assert.Equal(t, tt.wantPath, r.ko.Spec.Path)
			if tt.wantARN == nil {
				assert.Nil(t, r.ko.Status.ACKResourceMetadata)
			} else {
				assert.Equal(t, *tt.wantARN, string(*r.ko.Status.ACKResourceMetadata.ARN))
			}
		})
	}
}

func TestWithARNFromName(t *testing.T) {
	rm := &resourceManager{
		awsPartition: "aws-us-gov",
		awsAccountID: "123456789012",
	}
	tests := []struct {
		name        string
		specName    *string
		specPath    *string
		existingARN *string
		wantARN     *string
	}{
		{
			name:     "name uses default path",
			specName: aws.String("p"),
			wantARN:  aws.String("arn:aws-us-gov:iam::123456789012:policy/p"),
		},
		{
			name:     "name and path",
			specName: aws.String("p"),
			specPath: aws.String("/team/"),
			wantARN:  aws.String("arn:aws-us-gov:iam::123456789012:policy/team/p"),
		},
		{
			name:        "existing ARN is kept",
			specName:    aws.String("p"),
			existingARN: aws.String("arn:aws:iam::111111111111:policy/x"),
			wantARN:     aws.String("arn:aws:iam::111111111111:policy/x"),
		},
		{
			name: "no name",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := policyResource(nil, tt.specName, tt.specPath)
			if tt.existingARN != nil {
				arn := ackv1alpha1.AWSResourceName(*tt.existingARN)
				r.ko.Status.ACKResourceMetadata = &ackv1alpha1.ResourceMetadata{ARN: &arn}
			}
			before := r.ko.DeepCopy()
			got := rm.withARNFromName(r)
			assert.Equal(t, before, r.ko)
			if tt.wantARN == nil {
				assert.Nil(t, got.ko.Status.ACKResourceMetadata)
				return
			}
			assert.Equal(t, *tt.wantARN, string(*got.ko.Status.ACKResourceMetadata.ARN))
		})
	}
}
