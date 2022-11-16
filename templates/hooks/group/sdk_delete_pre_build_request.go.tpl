	// This causes syncPolicies to delete all associated policies from the group
	groupCpy := r.ko.DeepCopy()
	groupCpy.Spec.Policies = nil
	if err := rm.syncPolicies(ctx, &resource{ko: groupCpy}, r); err != nil {
		return nil, err
	}