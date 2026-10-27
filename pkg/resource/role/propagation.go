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
	"fmt"
	"time"

	ackcondition "github.com/aws-controllers-k8s/runtime/pkg/condition"
	ackrequeue "github.com/aws-controllers-k8s/runtime/pkg/requeue"
	ackrtlog "github.com/aws-controllers-k8s/runtime/pkg/runtime/log"
	acktypes "github.com/aws-controllers-k8s/runtime/pkg/types"
	corev1 "k8s.io/api/core/v1"

	svcapitypes "github.com/aws-controllers-k8s/iam-controller/apis/v1alpha1"
)

// readyAfterCreateWindow returns the propagation window requested by the
// AnnotationReadyAfterCreate annotation on the Role. It returns 0 when the
// annotation is absent, and 0 with an error when the value is not a valid,
// non-negative Go duration.
func readyAfterCreateWindow(ko *svcapitypes.Role) (time.Duration, error) {
	value, ok := ko.GetAnnotations()[svcapitypes.AnnotationReadyAfterCreate]
	if !ok {
		return 0, nil
	}
	window, err := time.ParseDuration(value)
	if err != nil {
		return 0, fmt.Errorf("invalid %s annotation value %q: %w",
			svcapitypes.AnnotationReadyAfterCreate, value, err)
	}
	if window < 0 {
		return 0, fmt.Errorf("invalid %s annotation value %q: must not be negative",
			svcapitypes.AnnotationReadyAfterCreate, value)
	}
	return window, nil
}

// propagationWaitRemaining returns how much longer a Role must wait, as of
// `now`, before it is reported as synced. It is 0 when no window is
// requested, when the role has no CreateDate yet, or when the role was
// created longer ago than the window (which also covers adopted roles).
func propagationWaitRemaining(ko *svcapitypes.Role, now time.Time) (time.Duration, error) {
	window, err := readyAfterCreateWindow(ko)
	if err != nil || window == 0 {
		return 0, err
	}
	if ko.Status.CreateDate == nil {
		return 0, nil
	}
	elapsed := now.Sub(ko.Status.CreateDate.Time)
	// CreateDate comes from IAM and `now` from the controller host. Clamp
	// clock skew so a CreateDate in the "future" can never extend the window.
	if elapsed < 0 {
		elapsed = 0
	}
	if elapsed >= window {
		return 0, nil
	}
	return window - elapsed, nil
}

// waitForPropagation holds ACK.ResourceSynced=False on a freshly created Role
// for the window requested by the AnnotationReadyAfterCreate annotation.
//
// It runs from LateInitialize, which the reconciler calls after any update
// (policy attachments, inline policies, ...) has been applied, so the delay
// never postpones those changes. It returns a nil error when no wait is
// needed; otherwise it returns `res` with the Synced condition set to False
// and a requeue for exactly the remaining part of the window.
func (rm *resourceManager) waitForPropagation(
	ctx context.Context,
	res acktypes.AWSResource,
) (acktypes.AWSResource, error) {
	ko := rm.concreteResource(res).ko
	remaining, err := propagationWaitRemaining(ko, time.Now())
	if err != nil {
		// An invalid annotation must not block the resource: log it and keep
		// today's behaviour.
		ackrtlog.FromContext(ctx).Info("ignoring annotation", "error", err.Error())
		return res, nil
	}
	if remaining == 0 {
		return res, nil
	}
	// Round up so the requeue lands just after the window, not just before.
	remaining = remaining.Truncate(time.Second) + time.Second
	msg := fmt.Sprintf(
		"Role created; waiting for IAM propagation (%s=%s), requeuing in %s",
		svcapitypes.AnnotationReadyAfterCreate,
		ko.GetAnnotations()[svcapitypes.AnnotationReadyAfterCreate],
		remaining,
	)
	reason := "Waiting for IAM propagation"
	ackcondition.SetSynced(res, corev1.ConditionFalse, &msg, &reason)
	return res, ackrequeue.NeededAfter(
		fmt.Errorf("waiting for IAM propagation after role creation"), remaining,
	)
}
