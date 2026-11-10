package open_id_connect_provider

import (
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/stretchr/testify/assert"

	svcapitypes "github.com/aws-controllers-k8s/iam-controller/apis/v1alpha1"
)

func newThumbprintsResource(thumbprints ...string) *resource {
	return &resource{
		ko: &svcapitypes.OpenIDConnectProvider{
			Spec: svcapitypes.OpenIDConnectProviderSpec{
				URL:         aws.String("https://example.com/"),
				Thumbprints: aws.StringSlice(thumbprints),
			},
		},
	}
}

func TestNewResourceDeltaThumbprints(t *testing.T) {
	const (
		thumbprint      = "3382517058a0c20228d598ee7501b61256a76442"
		otherThumbprint = "b5ee89e77326ab2bf1775bd99c19a28947ff8184"
	)

	tests := []struct {
		name     string
		desired  *resource
		latest   *resource
		expected bool
	}{
		{
			// An adopted provider, compared before its thumbprints are late initialized.
			name:     "empty desired thumbprints have no opinion",
			desired:  newThumbprintsResource(),
			latest:   newThumbprintsResource(thumbprint),
			expected: false,
		},
		{
			name:     "equal thumbprints",
			desired:  newThumbprintsResource(thumbprint),
			latest:   newThumbprintsResource(thumbprint),
			expected: false,
		},
		{
			name:     "changed thumbprint",
			desired:  newThumbprintsResource(otherThumbprint),
			latest:   newThumbprintsResource(thumbprint),
			expected: true,
		},
		{
			name:     "added thumbprint",
			desired:  newThumbprintsResource(thumbprint, otherThumbprint),
			latest:   newThumbprintsResource(thumbprint),
			expected: true,
		},
		{
			name:     "thumbprints AWS does not hold yet",
			desired:  newThumbprintsResource(thumbprint),
			latest:   newThumbprintsResource(),
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			delta := newResourceDelta(tt.desired, tt.latest)
			assert.Equal(t, tt.expected, delta.DifferentAt("Spec.Thumbprints"))
		})
	}
}
