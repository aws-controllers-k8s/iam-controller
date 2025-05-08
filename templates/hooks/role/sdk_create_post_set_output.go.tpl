    if ko.Spec.AssumeRolePolicyDocument != nil {
        if doc, err := decodeDocument(*ko.Spec.AssumeRolePolicyDocument); err != nil {
            return nil, err
        } else {
            ko.Spec.AssumeRolePolicyDocument = &doc
        }
    }
	for _, p := range desired.ko.Spec.Policies {
		err := rm.addManagedPolicy(ctx, &resource{ko}, p)
		if err != nil {
			return &resource{ko}, err
		}
	}
	for n, p := range desired.ko.Spec.InlinePolicies {
		err := rm.addInlinePolicy(ctx, &resource{ko}, n, p)
		if err != nil {
			return &resource{ko}, err
		}
	}
    ackcondition.SetSynced(&resource{ko}, corev1.ConditionFalse, nil, nil)
