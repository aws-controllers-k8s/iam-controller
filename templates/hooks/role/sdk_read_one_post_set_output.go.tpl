	if ko.Spec.AssumeRolePolicyDocument != nil {
		if doc, err := decodeAssumeDocument(*ko.Spec.AssumeRolePolicyDocument); err != nil {
			return nil, err
		} else {
			ko.Spec.AssumeRolePolicyDocument = &doc
		}
	}
	ko.Spec.Policies, err = rm.getPolicies(ctx, &resource{ko})
	if err != nil {
		return nil, err
	}
	ko.Spec.Tags, err = rm.getTags(ctx, &resource{ko})
	if err != nil {
		return nil, err
	}