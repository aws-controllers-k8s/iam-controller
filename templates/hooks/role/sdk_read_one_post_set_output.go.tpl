	if ko.Spec.AssumeRolePolicyDocument != nil {
		if doc, err := decodeDocument(*ko.Spec.AssumeRolePolicyDocument); err != nil {
			return nil, err
		} else {
			ko.Spec.AssumeRolePolicyDocument = &doc
		}
	}
	ko.Spec.Policies, err = rm.getManagedPolicies(ctx, &resource{ko})
	if err != nil {
		return nil, err
	}
	ko.Spec.InlinePolicies, err = rm.getInlinePolicies(ctx, &resource{ko})
	if err != nil {
		return nil, err
	}
	ko.Spec.Tags, err = rm.getTags(ctx, &resource{ko})
	if err != nil {
		return nil, err
	}
