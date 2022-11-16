	// This causes syncPolicies to create all associated policies to the group
	groupCpy := ko.DeepCopy()
	groupCpy.Spec.Policies = nil
	if err := rm.syncPolicies(ctx, desired, &resource{ko: groupCpy}); err != nil {
		return nil, err
	}