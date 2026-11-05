package policy

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"

	ackv1alpha1 "github.com/aws-controllers-k8s/runtime/apis/core/v1alpha1"
	ackcfg "github.com/aws-controllers-k8s/runtime/pkg/config"
	ackerr "github.com/aws-controllers-k8s/runtime/pkg/errors"
	ackmetrics "github.com/aws-controllers-k8s/runtime/pkg/metrics"
	"github.com/aws/aws-sdk-go-v2/aws"
	svcsdk "github.com/aws/aws-sdk-go-v2/service/iam"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	svcapitypes "github.com/aws-controllers-k8s/iam-controller/apis/v1alpha1"
)

const (
	testAccountID = "123456789012"
	testPolicyARN = "arn:aws:iam::123456789012:policy/test-policy"
)

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

func newOrphanTestResourceManager(fake *fakeIAMPolicyAPI) *resourceManager {
	return &resourceManager{
		cfg:          ackcfg.Config{Partition: "aws"},
		awsAccountID: testAccountID,
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

func TestOrphanedPolicyARN(t *testing.T) {
	arnOf := func(s string) *ackv1alpha1.AWSResourceName {
		arn := ackv1alpha1.AWSResourceName(s)
		return &arn
	}
	tests := []struct {
		name      string
		partition string
		account   string
		res       func() *resource
		want      *ackv1alpha1.AWSResourceName
	}{
		{
			name:      "managed without ARN: default path",
			partition: "aws",
			account:   testAccountID,
			res:       func() *resource { return policyWithoutARN(true) },
			want:      arnOf(testPolicyARN),
		},
		{
			name:      "managed without ARN: custom path and partition",
			partition: "aws-cn",
			account:   testAccountID,
			res: func() *resource {
				r := policyWithoutARN(true)
				r.ko.Spec.Path = aws.String("/team/app/")
				return r
			},
			want: arnOf("arn:aws-cn:iam::123456789012:policy/team/app/test-policy"),
		},
		{
			name:      "not managed: CreatePolicy was never called, or failed",
			partition: "aws",
			account:   testAccountID,
			res:       func() *resource { return policyWithoutARN(false) },
			want:      nil,
		},
		{
			name:      "ARN already in the status",
			partition: "aws",
			account:   testAccountID,
			res: func() *resource {
				r := policyWithoutARN(true)
				r.ko.Status.ACKResourceMetadata = &ackv1alpha1.ResourceMetadata{
					ARN: arnOf("arn:aws:iam::123456789012:policy/other"),
				}
				return r
			},
			want: nil,
		},
		{
			name:      "partition unknown",
			partition: "",
			account:   testAccountID,
			res:       func() *resource { return policyWithoutARN(true) },
			want:      nil,
		},
		{
			name:      "account unknown",
			partition: "aws",
			account:   "",
			res:       func() *resource { return policyWithoutARN(true) },
			want:      nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rm := &resourceManager{
				cfg:          ackcfg.Config{Partition: tt.partition},
				awsAccountID: ackv1alpha1.AWSAccountID(tt.account),
			}
			assert.Equal(t, tt.want, rm.orphanedPolicyARN(tt.res()))
		})
	}
}

// The controller created the policy and stopped before it saved the ARN. The
// next ReadOne finds the policy by the derived ARN instead of reporting
// NotFound, which would make the runtime call CreatePolicy again and fail with
// EntityAlreadyExists on every retry.
func TestReadOne_FindsManagedPolicyWithoutARN(t *testing.T) {
	document := `{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Action":"s3:GetObject","Resource":"*"}]}`
	fake := &fakeIAMPolicyAPI{policies: map[string]string{testPolicyARN: document}}
	rm := newOrphanTestResourceManager(fake)

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

// A managed resource without an ARN whose policy does not exist (the
// controller stopped before CreatePolicy) is NotFound, so the runtime creates
// it.
func TestReadOne_ManagedPolicyWithoutARNNotCreated(t *testing.T) {
	fake := &fakeIAMPolicyAPI{policies: map[string]string{}}
	rm := newOrphanTestResourceManager(fake)

	_, err := rm.ReadOne(context.Background(), policyWithoutARN(true))
	assert.Equal(t, ackerr.NotFound, err)
	require.Len(t, fake.requests, 1)
	assert.Equal(t, testPolicyARN, fake.requests[0].Get("PolicyArn"))
}

// A resource that is not managed yet is NotFound without any API call, as
// before: a policy with the same name created outside the controller is not
// taken over.
func TestReadOne_UnmanagedPolicyWithoutARN(t *testing.T) {
	fake := &fakeIAMPolicyAPI{policies: map[string]string{testPolicyARN: `{}`}}
	rm := newOrphanTestResourceManager(fake)

	_, err := rm.ReadOne(context.Background(), policyWithoutARN(false))
	assert.Equal(t, ackerr.NotFound, err)
	assert.Empty(t, fake.requests)
}
