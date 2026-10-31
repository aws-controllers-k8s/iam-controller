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
	ackcompare "github.com/aws-controllers-k8s/runtime/pkg/compare"
	ackmetrics "github.com/aws-controllers-k8s/runtime/pkg/metrics"
	"github.com/aws/aws-sdk-go-v2/aws"
	svcsdk "github.com/aws/aws-sdk-go-v2/service/iam"
	"github.com/stretchr/testify/assert"

	svcapitypes "github.com/aws-controllers-k8s/iam-controller/apis/v1alpha1"
)

// recordingHTTPClient is a fake IAM HTTP client. It records the query Action
// of every request so tests can assert which tag APIs were called.
type recordingHTTPClient struct {
	actions []string
}

func (c *recordingHTTPClient) Do(req *http.Request) (*http.Response, error) {
	body, err := io.ReadAll(req.Body)
	if err != nil {
		return nil, err
	}
	form, err := url.ParseQuery(string(body))
	if err != nil {
		return nil, err
	}
	action := form.Get("Action")
	c.actions = append(c.actions, action)

	respBody := fmt.Sprintf(
		`<%[1]sResponse xmlns="https://iam.amazonaws.com/doc/2010-05-08/"><ResponseMetadata><RequestId>test</RequestId></ResponseMetadata></%[1]sResponse>`,
		action,
	)
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"text/xml"}},
		Body:       io.NopCloser(strings.NewReader(respBody)),
		Request:    req,
	}, nil
}

// mutatingActions returns the recorded Tag and Untag actions, in call order.
func (c *recordingHTTPClient) mutatingActions() []string {
	var out []string
	for _, a := range c.actions {
		if strings.HasPrefix(a, "Tag") || strings.HasPrefix(a, "Untag") {
			out = append(out, a)
		}
	}
	return out
}

func newTestResourceManager(fake *recordingHTTPClient) *resourceManager {
	client := svcsdk.New(svcsdk.Options{
		Region:      "us-east-1",
		Credentials: aws.AnonymousCredentials{},
		HTTPClient:  fake,
		Retryer:     aws.NopRetryer{},
	})
	return &resourceManager{
		metrics: ackmetrics.NewMetrics("iam"),
		sdkapi:  client,
	}
}

func tag(key, value string) *svcapitypes.Tag {
	return &svcapitypes.Tag{Key: aws.String(key), Value: aws.String(value)}
}

func policyWithTags(tags ...*svcapitypes.Tag) *resource {
	return &resource{
		ko: &svcapitypes.Policy{
			Spec: svcapitypes.PolicySpec{
				Name: aws.String("test-policy"),
				Tags: tags,
			},
			Status: svcapitypes.PolicyStatus{
				ACKResourceMetadata: &ackv1alpha1.ResourceMetadata{
					ARN: (*ackv1alpha1.AWSResourceName)(
						aws.String("arn:aws:iam::123456789012:policy/test-policy"),
					),
				},
			},
		},
	}
}

func TestSyncTags(t *testing.T) {
	tests := []struct {
		name      string
		latest    []*svcapitypes.Tag
		desired   []*svcapitypes.Tag
		wantCalls []string
	}{
		{
			name:      "no diff makes no API calls",
			latest:    []*svcapitypes.Tag{tag("env", "prod")},
			desired:   []*svcapitypes.Tag{tag("env", "prod")},
			wantCalls: nil,
		},
		{
			name:      "new tag calls TagPolicy only",
			latest:    []*svcapitypes.Tag{tag("env", "prod")},
			desired:   []*svcapitypes.Tag{tag("env", "prod"), tag("team", "iam")},
			wantCalls: []string{"TagPolicy"},
		},
		{
			name:      "removed tag calls UntagPolicy only",
			latest:    []*svcapitypes.Tag{tag("env", "prod"), tag("team", "iam")},
			desired:   []*svcapitypes.Tag{tag("env", "prod")},
			wantCalls: []string{"UntagPolicy"},
		},
		{
			name:      "changed value calls TagPolicy only",
			latest:    []*svcapitypes.Tag{tag("env", "dev")},
			desired:   []*svcapitypes.Tag{tag("env", "prod")},
			wantCalls: []string{"TagPolicy"},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			fake := &recordingHTTPClient{}
			rm := newTestResourceManager(fake)

			err := rm.syncTags(
				context.Background(),
				policyWithTags(tc.desired...),
				policyWithTags(tc.latest...),
			)
			assert.NoError(t, err)
			assert.Equal(t, tc.wantCalls, fake.mutatingActions())
		})
	}
}

func TestCompareTags(t *testing.T) {
	tests := []struct {
		name     string
		a        []*svcapitypes.Tag
		b        []*svcapitypes.Tag
		wantDiff bool
	}{
		{
			name:     "same tags produce no delta",
			a:        []*svcapitypes.Tag{tag("env", "prod")},
			b:        []*svcapitypes.Tag{tag("env", "prod")},
			wantDiff: false,
		},
		{
			name:     "different value produces a delta",
			a:        []*svcapitypes.Tag{tag("env", "dev")},
			b:        []*svcapitypes.Tag{tag("env", "prod")},
			wantDiff: true,
		},
		{
			name:     "different key produces a delta",
			a:        []*svcapitypes.Tag{tag("env", "prod")},
			b:        []*svcapitypes.Tag{tag("team", "prod")},
			wantDiff: true,
		},
		{
			name:     "order of tags is ignored",
			a:        []*svcapitypes.Tag{tag("env", "prod"), tag("team", "iam")},
			b:        []*svcapitypes.Tag{tag("team", "iam"), tag("env", "prod")},
			wantDiff: false,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			delta := ackcompare.NewDelta()
			compareTags(delta, policyWithTags(tc.a...), policyWithTags(tc.b...))
			assert.Equal(t, tc.wantDiff, delta.DifferentAt("Spec.Tags"))
		})
	}
}
