	if policies, err := rm.getManagedPolicies(ctx, &resource{ko}); err != nil {
		return nil, err
	} else {
		ko.Spec.Policies = policies
	}
	ko.Spec.InlinePolicies, err = rm.getInlinePolicies(ctx, &resource{ko})
	if err != nil {
		return nil, err
	}
	if tags, err := rm.getTags(ctx, &resource{ko}); err != nil {
		return nil, err
	} else {
		ko.Spec.Tags = tags
	}
