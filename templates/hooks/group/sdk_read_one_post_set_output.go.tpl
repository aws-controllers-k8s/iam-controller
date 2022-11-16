	ko.Spec.Policies, err = rm.getPolicies(ctx, &resource{ko})
	if err != nil {
		return nil, err
	}