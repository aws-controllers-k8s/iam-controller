	// This causes syncPolicies to delete all associated policies from the
	// user
	r.ko.Spec.Policies = []*string{}
	if err := rm.syncPolicies(ctx, r); err != nil {
		return nil, err
	}
