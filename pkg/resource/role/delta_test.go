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
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/stretchr/testify/assert"

	svcapitypes "github.com/aws-controllers-k8s/iam-controller/apis/v1alpha1"
)

// helper to build a *resource with only AssumeRolePolicyDocument set.
func roleWithPolicy(doc string) *resource {
	return &resource{
		ko: &svcapitypes.Role{
			Spec: svcapitypes.RoleSpec{
				Name:                     aws.String("test-role"),
				AssumeRolePolicyDocument: aws.String(doc),
			},
		},
	}
}

func TestNewResourceDelta_AssumeRolePolicyDocument(t *testing.T) {
	tests := []struct {
		name     string
		desired  string
		latest   string
		wantDiff bool
	}{
		{
			name: "identical policies produce no diff",
			desired: `{
				"Version": "2012-10-17",
				"Statement": [{
					"Effect": "Allow",
					"Principal": {"Service": "eks.amazonaws.com"},
					"Action": "sts:AssumeRole"
				}]
			}`,
			latest: `{
				"Version": "2012-10-17",
				"Statement": [{
					"Effect": "Allow",
					"Principal": {"Service": "eks.amazonaws.com"},
					"Action": "sts:AssumeRole"
				}]
			}`,
			wantDiff: false,
		},
		{
			name:    "whitespace differences produce no diff",
			desired: `{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Principal":{"Service":"eks.amazonaws.com"},"Action":"sts:AssumeRole"}]}`,
			latest: `{
				"Version": "2012-10-17",
				"Statement": [
					{
						"Effect": "Allow",
						"Principal": {
							"Service": "eks.amazonaws.com"
						},
						"Action": "sts:AssumeRole"
					}
				]
			}`,
			wantDiff: false,
		},
		{
			name: "JSON key ordering differences produce no diff",
			desired: `{
				"Version": "2012-10-17",
				"Statement": [{
					"Action": "sts:AssumeRole",
					"Effect": "Allow",
					"Principal": {"Service": "eks.amazonaws.com"}
				}]
			}`,
			latest: `{
				"Version": "2012-10-17",
				"Statement": [{
					"Effect": "Allow",
					"Principal": {"Service": "eks.amazonaws.com"},
					"Action": "sts:AssumeRole"
				}]
			}`,
			wantDiff: false,
		},
		{
			name: "statement ordering differences produce no diff",
			desired: `{
				"Version": "2012-10-17",
				"Statement": [
					{"Sid": "1", "Effect": "Allow", "Principal": {"Service": "eks.amazonaws.com"}, "Action": "sts:AssumeRole"},
					{"Sid": "2", "Effect": "Allow", "Principal": {"Service": "ec2.amazonaws.com"}, "Action": "sts:AssumeRole"}
				]
			}`,
			latest: `{
				"Version": "2012-10-17",
				"Statement": [
					{"Sid": "2", "Effect": "Allow", "Principal": {"Service": "ec2.amazonaws.com"}, "Action": "sts:AssumeRole"},
					{"Sid": "1", "Effect": "Allow", "Principal": {"Service": "eks.amazonaws.com"}, "Action": "sts:AssumeRole"}
				]
			}`,
			wantDiff: false,
		},
		{
			name: "Principal.Service array order difference produces no diff (issue scenario)",
			desired: `{
				"Version": "2012-10-17",
				"Statement": [{
					"Effect": "Allow",
					"Principal": {
						"Service": ["ec2.amazonaws.com", "eks.amazonaws.com"]
					},
					"Action": "sts:AssumeRole"
				}]
			}`,
			latest: `{
				"Version": "2012-10-17",
				"Statement": [{
					"Effect": "Allow",
					"Principal": {
						"Service": ["eks.amazonaws.com", "ec2.amazonaws.com"]
					},
					"Action": "sts:AssumeRole"
				}]
			}`,
			wantDiff: false,
		},
		{
			name: "Condition key ordering difference produces no diff (issue scenario)",
			desired: `{
				"Version": "2012-10-17",
				"Statement": [{
					"Effect": "Allow",
					"Principal": {"Federated": "arn:aws:iam::123456789012:oidc-provider/oidc.eks.us-west-2.amazonaws.com/id/ABCDEF"},
					"Action": "sts:AssumeRoleWithWebIdentity",
					"Condition": {
						"StringEquals": {
							"oidc.eks.us-west-2.amazonaws.com/id/ABCDEF:aud": "sts.amazonaws.com",
							"oidc.eks.us-west-2.amazonaws.com/id/ABCDEF:sub": ["system:serviceaccount:kube-system:aws-node"]
						}
					}
				}]
			}`,
			latest: `{
				"Version": "2012-10-17",
				"Statement": [{
					"Effect": "Allow",
					"Principal": {"Federated": "arn:aws:iam::123456789012:oidc-provider/oidc.eks.us-west-2.amazonaws.com/id/ABCDEF"},
					"Action": "sts:AssumeRoleWithWebIdentity",
					"Condition": {
						"StringEquals": {
							"oidc.eks.us-west-2.amazonaws.com/id/ABCDEF:sub": ["system:serviceaccount:kube-system:aws-node"],
							"oidc.eks.us-west-2.amazonaws.com/id/ABCDEF:aud": "sts.amazonaws.com"
						}
					}
				}]
			}`,
			wantDiff: false,
		},
		{
			name: "full issue scenario - Service array order AND Condition key order differ",
			desired: `{
				"Version": "2012-10-17",
				"Statement": [
					{
						"Effect": "Allow",
						"Principal": {
							"Service": ["ec2.amazonaws.com", "eks.amazonaws.com"]
						},
						"Action": "sts:AssumeRole"
					},
					{
						"Effect": "Allow",
						"Principal": {
							"Federated": "arn:aws:iam::123456789012:oidc-provider/oidc.eks.us-west-2.amazonaws.com/id/ABCDEF"
						},
						"Action": "sts:AssumeRoleWithWebIdentity",
						"Condition": {
							"StringEquals": {
								"oidc.eks.us-west-2.amazonaws.com/id/ABCDEF:aud": "sts.amazonaws.com",
								"oidc.eks.us-west-2.amazonaws.com/id/ABCDEF:sub": [
									"system:serviceaccount:kube-system:aws-node",
									"system:serviceaccount:kube-system:cilium-operator",
									"system:serviceaccount:kube-system:vpc-resource-controller"
								]
							}
						}
					}
				]
			}`,
			latest: `{
				"Version": "2012-10-17",
				"Statement": [
					{
						"Effect": "Allow",
						"Principal": {
							"Service": ["eks.amazonaws.com", "ec2.amazonaws.com"]
						},
						"Action": "sts:AssumeRole"
					},
					{
						"Effect": "Allow",
						"Principal": {
							"Federated": "arn:aws:iam::123456789012:oidc-provider/oidc.eks.us-west-2.amazonaws.com/id/ABCDEF"
						},
						"Action": "sts:AssumeRoleWithWebIdentity",
						"Condition": {
							"StringEquals": {
								"oidc.eks.us-west-2.amazonaws.com/id/ABCDEF:sub": [
									"system:serviceaccount:kube-system:aws-node",
									"system:serviceaccount:kube-system:cilium-operator",
									"system:serviceaccount:kube-system:vpc-resource-controller"
								],
								"oidc.eks.us-west-2.amazonaws.com/id/ABCDEF:aud": "sts.amazonaws.com"
							}
						}
					}
				]
			}`,
			wantDiff: false,
		},
		{
			name: "Action as string vs array produces no diff",
			desired: `{
				"Version": "2012-10-17",
				"Statement": [{
					"Effect": "Allow",
					"Principal": {"Service": "eks.amazonaws.com"},
					"Action": "sts:AssumeRole"
				}]
			}`,
			latest: `{
				"Version": "2012-10-17",
				"Statement": [{
					"Effect": "Allow",
					"Principal": {"Service": "eks.amazonaws.com"},
					"Action": ["sts:AssumeRole"]
				}]
			}`,
			wantDiff: false,
		},
		{
			name: "actually different policies produce a diff",
			desired: `{
				"Version": "2012-10-17",
				"Statement": [{
					"Effect": "Allow",
					"Principal": {"Service": "eks.amazonaws.com"},
					"Action": "sts:AssumeRole"
				}]
			}`,
			latest: `{
				"Version": "2012-10-17",
				"Statement": [{
					"Effect": "Allow",
					"Principal": {"Service": "lambda.amazonaws.com"},
					"Action": "sts:AssumeRole"
				}]
			}`,
			wantDiff: true,
		},
		{
			name: "different Effect produces a diff",
			desired: `{
				"Version": "2012-10-17",
				"Statement": [{
					"Effect": "Allow",
					"Principal": {"Service": "eks.amazonaws.com"},
					"Action": "sts:AssumeRole"
				}]
			}`,
			latest: `{
				"Version": "2012-10-17",
				"Statement": [{
					"Effect": "Deny",
					"Principal": {"Service": "eks.amazonaws.com"},
					"Action": "sts:AssumeRole"
				}]
			}`,
			wantDiff: true,
		},
		{
			name: "additional statement produces a diff",
			desired: `{
				"Version": "2012-10-17",
				"Statement": [{
					"Effect": "Allow",
					"Principal": {"Service": "eks.amazonaws.com"},
					"Action": "sts:AssumeRole"
				}]
			}`,
			latest: `{
				"Version": "2012-10-17",
				"Statement": [
					{"Effect": "Allow", "Principal": {"Service": "eks.amazonaws.com"}, "Action": "sts:AssumeRole"},
					{"Effect": "Allow", "Principal": {"Service": "ec2.amazonaws.com"}, "Action": "sts:AssumeRole"}
				]
			}`,
			wantDiff: true,
		},
		{
			name: "different Condition values produce a diff",
			desired: `{
				"Version": "2012-10-17",
				"Statement": [{
					"Effect": "Allow",
					"Principal": {"Federated": "arn:aws:iam::123456789012:oidc-provider/oidc.eks.us-west-2.amazonaws.com/id/ABCDEF"},
					"Action": "sts:AssumeRoleWithWebIdentity",
					"Condition": {
						"StringEquals": {
							"oidc.eks.us-west-2.amazonaws.com/id/ABCDEF:sub": "system:serviceaccount:kube-system:aws-node"
						}
					}
				}]
			}`,
			latest: `{
				"Version": "2012-10-17",
				"Statement": [{
					"Effect": "Allow",
					"Principal": {"Federated": "arn:aws:iam::123456789012:oidc-provider/oidc.eks.us-west-2.amazonaws.com/id/ABCDEF"},
					"Action": "sts:AssumeRoleWithWebIdentity",
					"Condition": {
						"StringEquals": {
							"oidc.eks.us-west-2.amazonaws.com/id/ABCDEF:sub": "system:serviceaccount:kube-system:different-sa"
						}
					}
				}]
			}`,
			wantDiff: true,
		},
		{
			name:    "nil desired vs non-nil latest produces a diff",
			desired: "",
			latest: `{
				"Version": "2012-10-17",
				"Statement": [{
					"Effect": "Allow",
					"Principal": {"Service": "eks.amazonaws.com"},
					"Action": "sts:AssumeRole"
				}]
			}`,
			wantDiff: true,
		},
		{
			name:     "both nil produces no diff",
			desired:  "",
			latest:   "",
			wantDiff: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var a, b *resource

			if tc.desired == "" && tc.latest == "" {
				// Both nil AssumeRolePolicyDocument
				a = &resource{
					ko: &svcapitypes.Role{
						Spec: svcapitypes.RoleSpec{
							Name: aws.String("test-role"),
						},
					},
				}
				b = &resource{
					ko: &svcapitypes.Role{
						Spec: svcapitypes.RoleSpec{
							Name: aws.String("test-role"),
						},
					},
				}
			} else if tc.desired == "" {
				a = &resource{
					ko: &svcapitypes.Role{
						Spec: svcapitypes.RoleSpec{
							Name: aws.String("test-role"),
						},
					},
				}
				b = roleWithPolicy(tc.latest)
			} else if tc.latest == "" {
				a = roleWithPolicy(tc.desired)
				b = &resource{
					ko: &svcapitypes.Role{
						Spec: svcapitypes.RoleSpec{
							Name: aws.String("test-role"),
						},
					},
				}
			} else {
				a = roleWithPolicy(tc.desired)
				b = roleWithPolicy(tc.latest)
			}

			delta := newResourceDelta(a, b)

			if tc.wantDiff {
				assert.True(t, delta.DifferentAt("Spec.AssumeRolePolicyDocument"),
					"expected a diff at Spec.AssumeRolePolicyDocument but got none")
			} else {
				assert.False(t, delta.DifferentAt("Spec.AssumeRolePolicyDocument"),
					"expected no diff at Spec.AssumeRolePolicyDocument but got one")
			}
		})
	}
}
