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
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"

	ackerr "github.com/aws-controllers-k8s/runtime/pkg/errors"
	ackmetrics "github.com/aws-controllers-k8s/runtime/pkg/metrics"
	"github.com/aws/aws-sdk-go-v2/aws"
	svcsdk "github.com/aws/aws-sdk-go-v2/service/iam"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	svcapitypes "github.com/aws-controllers-k8s/iam-controller/apis/v1alpha1"
)

const testPolicyARN = "arn:aws:iam::123456789012:policy/test-policy"

// fakeIAMPolicyAPI is a fake IAM HTTP client that serves GetPolicy and
// GetPolicyVersion for the policies it holds, keyed by ARN, and records the
// query of every request.
type fakeIAMPolicyAPI struct {
	policies map[string]string // ARN -> policy document (JSON)
	requests []url.Values
}

func (c *fakeIAMPolicyAPI) Do(req *http.Request) (*http.Response, error) {
	body, err := io.ReadAll(req.Body)
	if err != nil {
		return nil, err
	}
	form, err := url.ParseQuery(string(body))
	if err != nil {
		return nil, err
	}
	c.requests = append(c.requests, form)

	action := form.Get("Action")
	arn := form.Get("PolicyArn")
	document, ok := c.policies[arn]
	if !ok {
		return xmlResponse(req, http.StatusNotFound, fmt.Sprintf(
			`<ErrorResponse xmlns="https://iam.amazonaws.com/doc/2010-05-08/"><Error><Type>Sender</Type><Code>NoSuchEntity</Code><Message>Policy %s does not exist or is not attachable.</Message></Error><RequestId>test</RequestId></ErrorResponse>`,
			arn,
		)), nil
	}
	switch action {
	case "GetPolicy":
		name := arn[strings.LastIndex(arn, "/")+1:]
		return xmlResponse(req, http.StatusOK, fmt.Sprintf(
			`<GetPolicyResponse xmlns="https://iam.amazonaws.com/doc/2010-05-08/"><GetPolicyResult><Policy><PolicyName>%s</PolicyName><DefaultVersionId>v1</DefaultVersionId><PolicyId>ANPATEST</PolicyId><Path>/</Path><Arn>%s</Arn><AttachmentCount>0</AttachmentCount><IsAttachable>true</IsAttachable></Policy></GetPolicyResult><ResponseMetadata><RequestId>test</RequestId></ResponseMetadata></GetPolicyResponse>`,
			name, arn,
		)), nil
	case "GetPolicyVersion":
		return xmlResponse(req, http.StatusOK, fmt.Sprintf(
			`<GetPolicyVersionResponse xmlns="https://iam.amazonaws.com/doc/2010-05-08/"><GetPolicyVersionResult><PolicyVersion><Document>%s</Document><VersionId>v1</VersionId><IsDefaultVersion>true</IsDefaultVersion></PolicyVersion></GetPolicyVersionResult><ResponseMetadata><RequestId>test</RequestId></ResponseMetadata></GetPolicyVersionResponse>`,
			url.QueryEscape(document),
		)), nil
	}
	return nil, fmt.Errorf("unexpected IAM action %q", action)
}

func xmlResponse(req *http.Request, status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Header:     http.Header{"Content-Type": []string{"text/xml"}},
		Body:       io.NopCloser(strings.NewReader(body)),
		Request:    req,
	}
}

func newFakeIAMResourceManager(fake *fakeIAMPolicyAPI) *resourceManager {
	return &resourceManager{
		awsPartition: "aws",
		awsAccountID: "123456789012",
		metrics:      ackmetrics.NewMetrics("iam"),
		sdkapi: svcsdk.New(svcsdk.Options{
			Region:      "us-east-1",
			Credentials: aws.AnonymousCredentials{},
			HTTPClient:  fake,
			Retryer:     aws.NopRetryer{},
		}),
	}
}

// policyWithoutARN returns a Policy whose status has no ARN. managed adds the
// finalizer the runtime puts on a resource right before it calls CreatePolicy.
func policyWithoutARN(managed bool) *resource {
	ko := &svcapitypes.Policy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-policy",
			Namespace: "default",
		},
		Spec: svcapitypes.PolicySpec{
			Name:           aws.String("test-policy"),
			PolicyDocument: aws.String(`{"Version":"2012-10-17","Statement":[]}`),
		},
	}
	if managed {
		ko.ObjectMeta.Finalizers = []string{FinalizerString}
	}
	return &resource{ko: ko}
}

// The controller created the policy, but the ARN never reached the status:
// the controller stopped right after CreatePolicy, or the status patch failed.
// The next ReadOne finds the policy by the ARN derived from its name instead
// of reporting NotFound, which would make the runtime call CreatePolicy again
// and fail with EntityAlreadyExists.
func TestReadOne_FindsManagedPolicyWithoutARN(t *testing.T) {
	document := `{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Action":"s3:GetObject","Resource":"*"}]}`
	fake := &fakeIAMPolicyAPI{policies: map[string]string{testPolicyARN: document}}
	rm := newFakeIAMResourceManager(fake)

	latest, err := rm.ReadOne(context.Background(), policyWithoutARN(true))
	require.NoError(t, err)

	ko := rm.concreteResource(latest).ko
	require.NotNil(t, ko.Status.ACKResourceMetadata)
	require.NotNil(t, ko.Status.ACKResourceMetadata.ARN)
	assert.Equal(t, testPolicyARN, string(*ko.Status.ACKResourceMetadata.ARN))
	require.NotNil(t, ko.Spec.PolicyDocument)
	assert.Equal(t, document, *ko.Spec.PolicyDocument)

	require.NotEmpty(t, fake.requests)
	assert.Equal(t, "GetPolicy", fake.requests[0].Get("Action"))
	assert.Equal(t, testPolicyARN, fake.requests[0].Get("PolicyArn"))
}

// A Policy without an ARN whose policy does not exist in AWS is NotFound, so
// the runtime creates it: a new resource, or a managed one whose controller
// stopped before CreatePolicy.
func TestReadOne_PolicyWithoutARNNotFound(t *testing.T) {
	for _, managed := range []bool{false, true} {
		t.Run(fmt.Sprintf("managed=%v", managed), func(t *testing.T) {
			fake := &fakeIAMPolicyAPI{policies: map[string]string{}}
			rm := newFakeIAMResourceManager(fake)

			_, err := rm.ReadOne(context.Background(), policyWithoutARN(managed))
			assert.Equal(t, ackerr.NotFound, err)
			require.Len(t, fake.requests, 1)
			assert.Equal(t, testPolicyARN, fake.requests[0].Get("PolicyArn"))
		})
	}
}

// A new resource whose name is taken by a policy that exists outside the
// controller finds that policy, like Role does. The runtime then stops with
// the NotManaged terminal condition, because the resource has no finalizer,
// instead of calling CreatePolicy and failing with EntityAlreadyExists.
func TestReadOne_FindsExistingPolicyForNewResource(t *testing.T) {
	fake := &fakeIAMPolicyAPI{policies: map[string]string{testPolicyARN: `{}`}}
	rm := newFakeIAMResourceManager(fake)

	latest, err := rm.ReadOne(context.Background(), policyWithoutARN(false))
	require.NoError(t, err)
	ko := rm.concreteResource(latest).ko
	require.NotNil(t, ko.Status.ACKResourceMetadata)
	assert.Equal(t, testPolicyARN, string(*ko.Status.ACKResourceMetadata.ARN))
}
