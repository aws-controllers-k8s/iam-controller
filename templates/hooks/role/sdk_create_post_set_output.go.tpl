    if ko.Spec.AssumeRolePolicyDocument != nil {
		if doc, err := decodeDocument(*ko.Spec.AssumeRolePolicyDocument); err != nil {
			return nil, err
		} else {
			ko.Spec.AssumeRolePolicyDocument = &doc
		}
	}
    ackcondition.SetSynced(&resource{ko}, corev1.ConditionFalse, nil, nil)
