package instance_profile

import (
	"context"
	"fmt"
	"strings"

	svcapitypes "github.com/aws-controllers-k8s/iam-controller/apis/v1alpha1"
	commonutil "github.com/aws-controllers-k8s/iam-controller/pkg/util"
	ackcompare "github.com/aws-controllers-k8s/runtime/pkg/compare"
	ackerr "github.com/aws-controllers-k8s/runtime/pkg/errors"
	ackrtlog "github.com/aws-controllers-k8s/runtime/pkg/runtime/log"
	svcsdk "github.com/aws/aws-sdk-go/service/iam"
)

// customUpdateInstanceProfile is the custom implementation for
// InstanceProfile resource's update operation.
func (rm *resourceManager) customUpdateInstanceProfile(
	ctx context.Context,
	desired *resource,
	latest *resource,
	delta *ackcompare.Delta,
) (updated *resource, err error) {
	rlog := ackrtlog.FromContext(ctx)
	exit := rlog.Trace("rm.customUpdateInstanceProfile")
	defer func() { exit(err) }()

	// Do not proceed with update if an immutable field was updated
	if immutableFieldChanges := rm.getImmutableFieldChanges(delta); len(immutableFieldChanges) > 0 {
		msg := fmt.Sprintf("Immutable Spec fields have been modified: %s", strings.Join(immutableFieldChanges, ","))
		return nil, ackerr.NewTerminalError(fmt.Errorf(msg))
	}

	ko := desired.ko.DeepCopy()

	if delta.DifferentAt("Spec.Tags") {
		if err = rm.syncTags(ctx, desired, latest); err != nil {
			return nil, err
		}
	}
	if delta.DifferentAt("Spec.Role") {
		if err = rm.syncRole(ctx, desired, latest); err != nil {
			return nil, err
		}
	}

	rm.setStatusDefaults(ko)
	return &resource{ko}, nil
}

// syncRole takes the delta between the desired role for the instance
// profile and the currently attached role. If a difference is found,
// the role will be synced to the desired value
func (rm *resourceManager) syncRole(
	ctx context.Context,
	desired *resource,
	latest *resource,
) (err error) {
	rlog := ackrtlog.FromContext(ctx)
	exit := rlog.Trace("rm.syncRole")
	defer func() { exit(err) }()

	// If no role is desired, detach any existing roles
	if desired.ko.Spec.Role == nil {
		if latest.ko.Spec.Role == nil {
			return nil
		}
		if err = rm.detachRole(ctx, latest); err != nil {
			return err
		}
		// Don't continue, nothing left to do
		return nil
	}

	// If the currently attached role and the desired role are different,
	// detach the existing role
	if latest.ko.Spec.Role != nil {
		if *desired.ko.Spec.Role == *latest.ko.Spec.Role {
			return nil
		}
		if err = rm.detachRole(ctx, latest); err != nil {
			return err
		}
	}

	err = rm.attachRole(ctx, desired)
	return err
}

// attachRole will attach a new IAM role to the instance profile
func (rm *resourceManager) attachRole(
	ctx context.Context,
	desired *resource,
) (err error) {
	rlog := ackrtlog.FromContext(ctx)
	exit := rlog.Trace("rm.attachRole")
	defer func() { exit(err) }()

	input := &svcsdk.AddRoleToInstanceProfileInput{}
	input.SetInstanceProfileName(*desired.ko.Spec.Name)
	input.SetRoleName(*desired.ko.Spec.Role)
	_, err = rm.sdkapi.AddRoleToInstanceProfileWithContext(ctx, input)
	rm.metrics.RecordAPICall("UPDATE", "AddRoleToInstanceProfile", err)
	return err
}

// detachRole will detach an existing IAM role from the instance profile
func (rm *resourceManager) detachRole(
	ctx context.Context,
	latest *resource,
) (err error) {
	rlog := ackrtlog.FromContext(ctx)
	exit := rlog.Trace("rm.detachRole")
	defer func() { exit(err) }()

	input := &svcsdk.RemoveRoleFromInstanceProfileInput{}
	input.SetInstanceProfileName(*latest.ko.Spec.Name)
	input.SetRoleName(*latest.ko.Spec.Role)
	_, err = rm.sdkapi.RemoveRoleFromInstanceProfileWithContext(ctx, input)
	rm.metrics.RecordAPICall("UPDATE", "RemoveRoleFromInstanceProfile", err)
	return err
}

// syncTags examines the Tags in the supplied InstanceProfile and calls TagInstanceProfile
// and UntagInstanceProfile APIs to ensure that the set of associated Tags stays in sync
// with InstanceProfile.Spec.Tags
func (rm *resourceManager) syncTags(
	ctx context.Context,
	desired *resource,
	latest *resource,
) (err error) {
	rlog := ackrtlog.FromContext(ctx)
	exit := rlog.Trace("rm.syncTags")
	defer func() { exit(err) }()

	toAdd := []*svcapitypes.Tag{}
	toDelete := []*svcapitypes.Tag{}
	existingTags := latest.ko.Spec.Tags

	for _, t := range existingTags {
		if !inTags(*t.Key, *t.Value, desired.ko.Spec.Tags) {
			toDelete = append(toDelete, t)
		}
	}
	for _, t := range desired.ko.Spec.Tags {
		if !inTags(*t.Key, *t.Value, existingTags) {
			toAdd = append(toAdd, t)
		}
	}

	if len(toDelete) > 0 {
		for _, t := range toDelete {
			rlog.Debug("removing tag from instance profile", "key", *t.Key, "value", *t.Value)
		}
		if err = rm.removeTags(ctx, desired, toDelete); err != nil {
			return err
		}
	}
	if len(toAdd) > 0 {
		for _, t := range toAdd {
			rlog.Debug("adding tag to instance profile", "key", *t.Key, "value", *t.Value)
		}
		if err = rm.addTags(ctx, desired, toAdd); err != nil {
			return err
		}
	}

	return nil
}

// inTags returns true if the supplied key and value can be found in the
// supplied list of Tag structs.
func inTags(
	key string,
	value string,
	tags []*svcapitypes.Tag,
) bool {
	for _, t := range tags {
		if *t.Key == key && *t.Value == value {
			return true
		}
	}
	return false
}

// addTags adds the supplied Tags to the supplied InstanceProfile resource
func (rm *resourceManager) addTags(
	ctx context.Context,
	r *resource,
	tags []*svcapitypes.Tag,
) (err error) {
	rlog := ackrtlog.FromContext(ctx)
	exit := rlog.Trace("rm.addTag")
	defer func() { exit(err) }()

	input := &svcsdk.TagInstanceProfileInput{}
	input.InstanceProfileName = r.ko.Spec.Name
	inTags := []*svcsdk.Tag{}

	for _, t := range tags {
		inTags = append(inTags, &svcsdk.Tag{Key: t.Key, Value: t.Value})
	}
	input.Tags = inTags

	_, err = rm.sdkapi.TagInstanceProfileWithContext(ctx, input)
	rm.metrics.RecordAPICall("UPDATE", "TagInstanceProfile", err)
	return err
}

// removeTags removes the supplied Tags from the supplied InstanceProfile resource
func (rm *resourceManager) removeTags(
	ctx context.Context,
	r *resource,
	tags []*svcapitypes.Tag,
) (err error) {
	rlog := ackrtlog.FromContext(ctx)
	exit := rlog.Trace("rm.removeTag")
	defer func() { exit(err) }()

	input := &svcsdk.UntagInstanceProfileInput{}
	input.InstanceProfileName = r.ko.Spec.Name
	inTagKeys := []*string{}

	for _, t := range tags {
		inTagKeys = append(inTagKeys, t.Key)
	}
	input.TagKeys = inTagKeys

	_, err = rm.sdkapi.UntagInstanceProfileWithContext(ctx, input)
	rm.metrics.RecordAPICall("UPDATE", "UntagInstanceProfile", err)
	return err
}

// compareTags is a custom comparison function for comparing lists of Tag
// structs where the order of the structs in the list is not important.
func compareTags(
	delta *ackcompare.Delta,
	a *resource,
	b *resource,
) {
	if len(a.ko.Spec.Tags) != len(b.ko.Spec.Tags) {
		delta.Add("Spec.Tags", a.ko.Spec.Tags, b.ko.Spec.Tags)
	} else if len(a.ko.Spec.Tags) > 0 {
		if !commonutil.EqualTags(a.ko.Spec.Tags, b.ko.Spec.Tags) {
			delta.Add("Spec.Tags", a.ko.Spec.Tags, b.ko.Spec.Tags)
		}
	}
}
