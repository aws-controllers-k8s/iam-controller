	// If tags are specified, mark the resource as needing a sync
	if ko.Spec.Tags != nil && len(ko.Spec.Tags) > 0 {
		// Set the resource as needing a sync
		ackcondition.SetSynced(&resource{ko}, corev1.ConditionFalse, nil, nil)
	}