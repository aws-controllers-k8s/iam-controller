    if ko.Spec.AssumeRolePolicyDocument != nil {
		if doc, err := decodeAssumeDocument(*ko.Spec.AssumeRolePolicyDocument); err != nil {
			return nil, err
		} else {
			ko.Spec.AssumeRolePolicyDocument = &doc
		}
	}
    if err := rm.syncPolicies(ctx, &resource{ko}); err != nil {
        return nil, err
    }
    // There really isn't a status of a role... it either exists or doesn't. If
    // we get here, that means the creation was successful and the desired
    // state of the role matches what we provided...
    ackcondition.SetSynced(&resource{ko}, corev1.ConditionTrue, nil, nil)
