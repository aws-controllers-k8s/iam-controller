	// This causes syncPolicies to delete all associated policies from the role
	r.ko.Spec.Policies = []*string{}
	if err := rm.syncPolicies(ctx, r); err != nil {
		return nil, err
	}
