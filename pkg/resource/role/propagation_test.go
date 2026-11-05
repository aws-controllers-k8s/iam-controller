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
	"context"
	"errors"
	"testing"
	"time"

	ackcondition "github.com/aws-controllers-k8s/runtime/pkg/condition"
	ackrequeue "github.com/aws-controllers-k8s/runtime/pkg/requeue"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	svcapitypes "github.com/aws-controllers-k8s/iam-controller/apis/v1alpha1"
)

func newTestRole(annotation *string, createDate *time.Time) *svcapitypes.Role {
	ko := &svcapitypes.Role{}
	if annotation != nil {
		ko.SetAnnotations(map[string]string{
			svcapitypes.AnnotationReadyAfterCreate: *annotation,
		})
	}
	if createDate != nil {
		ko.Status.CreateDate = &metav1.Time{Time: *createDate}
	}
	return ko
}

func ptrTo[T any](v T) *T { return &v }

func TestPropagationWaitRemaining(t *testing.T) {
	now := time.Date(2026, 10, 27, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name       string
		annotation *string
		createDate *time.Time
		want       time.Duration
		wantErr    bool
	}{
		{
			name:       "no annotation keeps today's behaviour",
			createDate: ptrTo(now.Add(-1 * time.Second)),
			want:       0,
		},
		{
			name:       "window not elapsed returns the remainder",
			annotation: ptrTo("15s"),
			createDate: ptrTo(now.Add(-4 * time.Second)),
			want:       11 * time.Second,
		},
		{
			name:       "window elapsed",
			annotation: ptrTo("15s"),
			createDate: ptrTo(now.Add(-15 * time.Second)),
			want:       0,
		},
		{
			name:       "adopted role created long ago is not delayed",
			annotation: ptrTo("15s"),
			createDate: ptrTo(now.Add(-72 * time.Hour)),
			want:       0,
		},
		{
			name:       "no CreateDate yet",
			annotation: ptrTo("15s"),
			want:       0,
		},
		{
			name:       "CreateDate in the future (clock skew) never extends the window",
			annotation: ptrTo("15s"),
			createDate: ptrTo(now.Add(30 * time.Second)),
			want:       15 * time.Second,
		},
		{
			name:       "zero window",
			annotation: ptrTo("0s"),
			createDate: ptrTo(now),
			want:       0,
		},
		{
			name:       "invalid value",
			annotation: ptrTo("fifteen"),
			createDate: ptrTo(now),
			want:       0,
			wantErr:    true,
		},
		{
			name:       "negative value",
			annotation: ptrTo("-5s"),
			createDate: ptrTo(now),
			want:       0,
			wantErr:    true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := propagationWaitRemaining(newTestRole(tt.annotation, tt.createDate), now)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestWaitForPropagation(t *testing.T) {
	rm := &resourceManager{}
	ctx := context.Background()

	t.Run("holds Synced=False and requeues for the remainder", func(t *testing.T) {
		res := &resource{newTestRole(ptrTo("15s"), ptrTo(time.Now().Add(-5*time.Second)))}

		got, err := rm.waitForPropagation(ctx, res)

		var requeueAfter *ackrequeue.RequeueNeededAfter
		require.True(t, errors.As(err, &requeueAfter), "expected a RequeueNeededAfter, got %v", err)
		assert.Greater(t, requeueAfter.Duration(), 9*time.Second)
		assert.LessOrEqual(t, requeueAfter.Duration(), 11*time.Second)

		synced := ackcondition.Synced(got)
		require.NotNil(t, synced)
		assert.Equal(t, corev1.ConditionFalse, synced.Status)
		require.NotNil(t, synced.Message)
		assert.Contains(t, *synced.Message, "waiting for IAM propagation")
	})

	t.Run("window elapsed leaves the resource untouched", func(t *testing.T) {
		res := &resource{newTestRole(ptrTo("15s"), ptrTo(time.Now().Add(-1*time.Minute)))}

		got, err := rm.waitForPropagation(ctx, res)

		assert.NoError(t, err)
		assert.Nil(t, ackcondition.Synced(got))
	})

	t.Run("no annotation leaves the resource untouched", func(t *testing.T) {
		res := &resource{newTestRole(nil, ptrTo(time.Now()))}

		got, err := rm.waitForPropagation(ctx, res)

		assert.NoError(t, err)
		assert.Nil(t, ackcondition.Synced(got))
	})

	t.Run("invalid annotation is ignored instead of blocking", func(t *testing.T) {
		res := &resource{newTestRole(ptrTo("soon"), ptrTo(time.Now()))}

		got, err := rm.waitForPropagation(ctx, res)

		assert.NoError(t, err)
		assert.Nil(t, ackcondition.Synced(got))
	})
}
