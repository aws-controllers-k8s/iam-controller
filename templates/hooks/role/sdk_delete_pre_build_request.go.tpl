	// This causes syncPolicies to delete all associated policies from the role
	roleCpy := r.ko.DeepCopy()
	roleCpy.Spec.Policies = nil
	if err := rm.syncPolicies(ctx, &resource{ko: roleCpy}, r); err != nil {
		return nil, err
	}