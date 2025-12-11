	if delta.DifferentAt("Spec.Policies") {
		err = rm.syncManagedPolicies(ctx, desired, latest)
		if err != nil {
			return nil, err
		}
	}
	if delta.DifferentAt("Spec.InlinePolicies") {
		err = rm.syncInlinePolicies(ctx, desired, latest)
		if err != nil {
			return nil, err
		}
	}
	if delta.DifferentAt("Spec.Users") {
		err = rm.syncUsers(ctx, desired, latest)
		if err != nil {
			return nil, err
		}
	}
	if !delta.DifferentExcept("Spec.Tags", "Spec.Policies", "Spec.InlinePolicies", "Spec.Users", "Spec.PermissionsBoundary") {
		return desired, nil
	}
