	// This causes syncPolicies to delete all associated policies from the user
	userCpy := r.ko.DeepCopy()
	userCpy.Spec.Policies = nil
	if err := rm.syncPolicies(ctx, &resource{ko: userCpy}, r); err != nil {
		return nil, err
	}