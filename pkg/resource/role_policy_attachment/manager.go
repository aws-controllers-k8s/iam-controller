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

package role_policy_attachment

import (
	"context"
	"fmt"
	"time"

	ackv1alpha1 "github.com/aws-controllers-k8s/runtime/apis/core/v1alpha1"
	ackcompare "github.com/aws-controllers-k8s/runtime/pkg/compare"
	ackcondition "github.com/aws-controllers-k8s/runtime/pkg/condition"
	ackcfg "github.com/aws-controllers-k8s/runtime/pkg/config"
	ackerr "github.com/aws-controllers-k8s/runtime/pkg/errors"
	ackmetrics "github.com/aws-controllers-k8s/runtime/pkg/metrics"
	ackrequeue "github.com/aws-controllers-k8s/runtime/pkg/requeue"
	acktypes "github.com/aws-controllers-k8s/runtime/pkg/types"
	"github.com/aws/aws-sdk-go-v2/aws"
	svcsdk "github.com/aws/aws-sdk-go-v2/service/iam"
	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
)

// +kubebuilder:rbac:groups=iam.services.k8s.aws,resources=rolepolicyattachments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=iam.services.k8s.aws,resources=rolepolicyattachments/status,verbs=get;update;patch

var lateInitializeFieldNames = []string{}

type resourceManager struct {
	cfg          ackcfg.Config
	clientcfg    aws.Config
	log          logr.Logger
	metrics      *ackmetrics.Metrics
	rr           acktypes.Reconciler
	awsAccountID ackv1alpha1.AWSAccountID
	awsRegion    ackv1alpha1.AWSRegion
	awsPartition ackv1alpha1.AWSPartition
	sdkapi       *svcsdk.Client
}

func (rm *resourceManager) concreteResource(res acktypes.AWSResource) *resource {
	return res.(*resource)
}

func (rm *resourceManager) ReadOne(ctx context.Context, res acktypes.AWSResource) (acktypes.AWSResource, error) {
	r := rm.concreteResource(res)
	if r.ko == nil {
		panic("resource manager's ReadOne() method received resource with nil CR object")
	}
	observed, err := rm.sdkFind(ctx, r)
	if err != nil {
		if observed != nil {
			return rm.onError(observed, err)
		}
		return rm.onError(r, err)
	}
	return rm.onSuccess(observed)
}

func (rm *resourceManager) Create(ctx context.Context, res acktypes.AWSResource) (acktypes.AWSResource, error) {
	r := rm.concreteResource(res)
	if r.ko == nil {
		panic("resource manager's Create() method received resource with nil CR object")
	}
	created, err := rm.sdkCreate(ctx, r)
	if err != nil {
		if created != nil {
			return rm.onError(created, err)
		}
		return rm.onError(r, err)
	}
	return rm.onSuccess(created)
}

func (rm *resourceManager) Update(
	ctx context.Context,
	resDesired acktypes.AWSResource,
	resLatest acktypes.AWSResource,
	delta *ackcompare.Delta,
) (acktypes.AWSResource, error) {
	desired := rm.concreteResource(resDesired)
	latest := rm.concreteResource(resLatest)
	if desired.ko == nil || latest.ko == nil {
		panic("resource manager's Update() method received resource with nil CR object")
	}
	updated, err := rm.sdkUpdate(ctx, desired, latest, delta)
	if err != nil {
		if updated != nil {
			return rm.onError(updated, err)
		}
		return rm.onError(latest, err)
	}
	return rm.onSuccess(updated)
}

func (rm *resourceManager) Delete(ctx context.Context, res acktypes.AWSResource) (acktypes.AWSResource, error) {
	r := rm.concreteResource(res)
	if r.ko == nil {
		panic("resource manager's Delete() method received resource with nil CR object")
	}
	observed, err := rm.sdkDelete(ctx, r)
	if err != nil {
		if observed != nil {
			return rm.onError(observed, err)
		}
		return rm.onError(r, err)
	}
	return rm.onSuccess(observed)
}

func (rm *resourceManager) ARNFromName(name string) string {
	return fmt.Sprintf("arn:%s:iam:%s:%s:%s", rm.awsPartition, rm.awsRegion, rm.awsAccountID, name)
}

func (rm *resourceManager) LateInitialize(ctx context.Context, latest acktypes.AWSResource) (acktypes.AWSResource, error) {
	if len(lateInitializeFieldNames) == 0 {
		return latest, nil
	}
	latestCopy := latest.DeepCopy()
	lateInitConditionReason := ""
	lateInitConditionMessage := ""
	observed, err := rm.ReadOne(ctx, latestCopy)
	if err != nil {
		lateInitConditionMessage = "Unable to complete Read operation required for late initialization"
		lateInitConditionReason = "Late Initialization Failure"
		ackcondition.SetLateInitialized(latestCopy, corev1.ConditionFalse, &lateInitConditionMessage, &lateInitConditionReason)
		ackcondition.SetSynced(latestCopy, corev1.ConditionFalse, nil, nil)
		return latestCopy, err
	}
	lateInitializedRes := rm.lateInitializeFromReadOneOutput(observed, latestCopy)
	if rm.incompleteLateInitialization(lateInitializedRes) {
		lateInitConditionMessage = "Late initialization did not complete, requeuing with delay of 5 seconds"
		lateInitConditionReason = "Delayed Late Initialization"
		ackcondition.SetLateInitialized(lateInitializedRes, corev1.ConditionFalse, &lateInitConditionMessage, &lateInitConditionReason)
		ackcondition.SetSynced(lateInitializedRes, corev1.ConditionFalse, nil, nil)
		return lateInitializedRes, ackrequeue.NeededAfter(nil, 5*time.Second)
	}
	lateInitConditionMessage = "Late initialization successful"
	lateInitConditionReason = "Late initialization successful"
	ackcondition.SetLateInitialized(lateInitializedRes, corev1.ConditionTrue, &lateInitConditionMessage, &lateInitConditionReason)
	return lateInitializedRes, nil
}

func (rm *resourceManager) incompleteLateInitialization(res acktypes.AWSResource) bool {
	return false
}

func (rm *resourceManager) lateInitializeFromReadOneOutput(observed acktypes.AWSResource, latest acktypes.AWSResource) acktypes.AWSResource {
	return latest
}

func (rm *resourceManager) IsSynced(ctx context.Context, res acktypes.AWSResource) (bool, error) {
	return true, nil
}

func (rm *resourceManager) EnsureTags(ctx context.Context, res acktypes.AWSResource, md acktypes.ServiceControllerMetadata) error {
	return nil
}

func (rm *resourceManager) FilterSystemTags(res acktypes.AWSResource, systemTags []string) {}

func newResourceManager(
	cfg ackcfg.Config,
	clientcfg aws.Config,
	log logr.Logger,
	metrics *ackmetrics.Metrics,
	rr acktypes.Reconciler,
	id ackv1alpha1.AWSAccountID,
	region ackv1alpha1.AWSRegion,
) (*resourceManager, error) {
	return &resourceManager{
		cfg:          cfg,
		clientcfg:    clientcfg,
		log:          log,
		metrics:      metrics,
		rr:           rr,
		awsAccountID: id,
		awsRegion:    region,
		awsPartition: ackv1alpha1.AWSPartition(cfg.Partition),
		sdkapi:       svcsdk.NewFromConfig(clientcfg),
	}, nil
}

func (rm *resourceManager) onError(r *resource, err error) (acktypes.AWSResource, error) {
	if r == nil {
		return nil, err
	}
	r1, updated := rm.updateConditions(r, false, err)
	if !updated {
		return r, err
	}
	for _, condition := range r1.Conditions() {
		if condition.Type == ackv1alpha1.ConditionTypeTerminal && condition.Status == corev1.ConditionTrue {
			return r1, ackerr.Terminal
		}
	}
	return r1, err
}

func (rm *resourceManager) onSuccess(r *resource) (acktypes.AWSResource, error) {
	if r == nil {
		return nil, nil
	}
	r1, updated := rm.updateConditions(r, true, nil)
	if !updated {
		return r, nil
	}
	return r1, nil
}
