	if policies, err := rm.getPolicies(ctx, &resource{ko}); err != nil {
		return nil, err
	} else {
		ko.Spec.Policies = policies
	}
