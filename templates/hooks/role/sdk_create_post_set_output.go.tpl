    if ko.Spec.AssumeRolePolicyDocument != nil {
        if doc, err := decodeDocument(*ko.Spec.AssumeRolePolicyDocument); err != nil {
            return nil, err
        } else {
            ko.Spec.AssumeRolePolicyDocument = &doc
        }
    }
	err = rm.attachPolicies(ctx, &resource{ko})
	if err != nil {
		return &resource{ko}, err
	}
    ackcondition.SetSynced(&resource{ko}, corev1.ConditionFalse, nil, nil)
