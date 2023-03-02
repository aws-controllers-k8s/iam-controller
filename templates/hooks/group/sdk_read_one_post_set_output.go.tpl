	ko.Spec.Policies, err = rm.getManagedPolicies(ctx, &resource{ko})
	if err != nil {
		return nil, err
	}
	ko.Spec.InlinePolicies, err = rm.getInlinePolicies(ctx, &resource{ko})
	if err != nil {
		return nil, err
	}
