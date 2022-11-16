	// This causes syncPolicies to create all associated policies to the user
	userCpy := ko.DeepCopy()
	userCpy.Spec.Policies = nil
	if err := rm.syncPolicies(ctx, desired, &resource{ko: userCpy}); err != nil {
		return nil, err
	}