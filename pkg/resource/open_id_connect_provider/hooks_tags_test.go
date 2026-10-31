package open_id_connect_provider

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
// of every request so tests can assert which tag APIs were called. Tags
// currently attached to the provider are returned by the List call.
type recordingHTTPClient struct {
	actions      []string
	existingTags []*svcapitypes.Tag
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
	if strings.HasPrefix(action, "List") {
		respBody = listTagsResponse(action, c.existingTags)
	}
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"text/xml"}},
		Body:       io.NopCloser(strings.NewReader(respBody)),
		Request:    req,
	}, nil
}

// listTagsResponse builds the XML body of a ListOpenIDConnectProviderTags
// response containing the supplied tags.
func listTagsResponse(action string, tags []*svcapitypes.Tag) string {
	var members strings.Builder
	for _, t := range tags {
		fmt.Fprintf(&members, "<member><Key>%s</Key><Value>%s</Value></member>", *t.Key, *t.Value)
	}
	return fmt.Sprintf(
		`<%[1]sResponse xmlns="https://iam.amazonaws.com/doc/2010-05-08/"><%[1]sResult><IsTruncated>false</IsTruncated><Tags>%[2]s</Tags></%[1]sResult><ResponseMetadata><RequestId>test</RequestId></ResponseMetadata></%[1]sResponse>`,
		action, members.String(),
	)
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

func oidcWithTags(tags ...*svcapitypes.Tag) *resource {
	return &resource{
		ko: &svcapitypes.OpenIDConnectProvider{
			Spec: svcapitypes.OpenIDConnectProviderSpec{
				Tags: tags,
			},
			Status: svcapitypes.OpenIDConnectProviderStatus{
				ACKResourceMetadata: &ackv1alpha1.ResourceMetadata{
					ARN: (*ackv1alpha1.AWSResourceName)(
						aws.String("arn:aws:iam::123456789012:oidc-provider/example.com"),
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
			name:      "new tag calls TagOpenIDConnectProvider only",
			latest:    []*svcapitypes.Tag{tag("env", "prod")},
			desired:   []*svcapitypes.Tag{tag("env", "prod"), tag("team", "iam")},
			wantCalls: []string{"TagOpenIDConnectProvider"},
		},
		{
			name:      "removed tag calls UntagOpenIDConnectProvider only",
			latest:    []*svcapitypes.Tag{tag("env", "prod"), tag("team", "iam")},
			desired:   []*svcapitypes.Tag{tag("env", "prod")},
			wantCalls: []string{"UntagOpenIDConnectProvider"},
		},
		{
			name:      "changed value calls TagOpenIDConnectProvider only",
			latest:    []*svcapitypes.Tag{tag("env", "dev")},
			desired:   []*svcapitypes.Tag{tag("env", "prod")},
			wantCalls: []string{"TagOpenIDConnectProvider"},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			fake := &recordingHTTPClient{existingTags: tc.latest}
			rm := newTestResourceManager(fake)

			err := rm.syncTags(context.Background(), oidcWithTags(tc.desired...))
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
			customPreCompare(delta, oidcWithTags(tc.a...), oidcWithTags(tc.b...))
			assert.Equal(t, tc.wantDiff, delta.DifferentAt("Spec.Tags"))
		})
	}
}
