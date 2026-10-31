package instance_profile

import (
	"context"

	svcapitypes "github.com/aws-controllers-k8s/iam-controller/apis/v1alpha1"
	ackcompare "github.com/aws-controllers-k8s/runtime/pkg/compare"
	ackrtlog "github.com/aws-controllers-k8s/runtime/pkg/runtime/log"
	svcsdk "github.com/aws/aws-sdk-go-v2/service/iam"
	svcsdktypes "github.com/aws/aws-sdk-go-v2/service/iam/types"
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
	input.InstanceProfileName = desired.ko.Spec.Name
	input.RoleName = desired.ko.Spec.Role
	_, err = rm.sdkapi.AddRoleToInstanceProfile(ctx, input)
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
	input.InstanceProfileName = latest.ko.Spec.Name
	input.RoleName = latest.ko.Spec.Role
	_, err = rm.sdkapi.RemoveRoleFromInstanceProfile(ctx, input)
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

	fromTags, _ := convertToOrderedACKTags(latest.ko.Spec.Tags)
	toTags, toOrder := convertToOrderedACKTags(desired.ko.Spec.Tags)
	// A tag whose value changed lands in added only (upsert), so it is not untagged first.
	// The discarded middle return is the "unchanged" set (same key and value in
	// both inputs); those tags need no AWS API call, so it is ignored.
	added, _, removed := ackcompare.GetTagsDifference(fromTags, toTags)
	toAdd := fromACKTags(added, toOrder)
	toRemove := fromACKTags(removed, nil)

	// Remove before add, intentionally: this order is kept from the previous
	// implementation, where a changed value appeared in both lists and had to
	// be untagged first. GetTagsDifference keeps the key sets of added and
	// removed disjoint, so the order no longer changes the final tag set; it is
	// kept to avoid a behaviour change and to free tag quota before adding.
	if len(toRemove) > 0 {
		for _, t := range toRemove {
			rlog.Debug("removing tag from instance profile", "key", *t.Key, "value", *t.Value)
		}
		if err = rm.removeTags(ctx, desired, toRemove); err != nil {
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
	inTags := []svcsdktypes.Tag{}

	for _, t := range tags {
		inTags = append(inTags, svcsdktypes.Tag{Key: t.Key, Value: t.Value})
	}
	input.Tags = inTags

	_, err = rm.sdkapi.TagInstanceProfile(ctx, input)
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
	inTagKeys := []string{}

	for _, t := range tags {
		inTagKeys = append(inTagKeys, *t.Key)
	}
	input.TagKeys = inTagKeys

	_, err = rm.sdkapi.UntagInstanceProfile(ctx, input)
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
	fromTags, _ := convertToOrderedACKTags(a.ko.Spec.Tags)
	toTags, _ := convertToOrderedACKTags(b.ko.Spec.Tags)
	// The discarded middle return is the "unchanged" set (same key and value in
	// both inputs); those tags need no AWS API call, so it is ignored.
	added, _, removed := ackcompare.GetTagsDifference(fromTags, toTags)
	if len(added) > 0 || len(removed) > 0 {
		delta.Add("Spec.Tags", a.ko.Spec.Tags, b.ko.Spec.Tags)
	}
}
