	if policies, err := rm.getPolicies(ctx, &resource{ko}); err != nil {
		return nil, err
	} else {
		ko.Spec.Policies = policies
	}
	if tags, err := rm.getTags(ctx, &resource{ko}); err != nil {
		return nil, err
	} else {
		ko.Spec.Tags = tags
	}
