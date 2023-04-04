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
	if delta.DifferentAt("Spec.Tags") {
		err = rm.syncTags(ctx, desired, latest)
		if err != nil {
			return nil, err
		}
	}
	if delta.DifferentAt("Spec.PermissionsBoundary") {
		err = rm.syncRolePermissionsBoundary(ctx, desired)
		if err != nil {
			return nil, err
		}
	}
	if delta.DifferentAt("Spec.AssumeRolePolicyDocument") {
		err = rm.putAssumeRolePolicy(ctx, desired)
		if err != nil {
			return nil, err
		}
	}
	if !delta.DifferentExcept("Spec.Tags", "Spec.Policies", "Spec.InlinePolicies", "Spec.PermissionsBoundary", "Spec.AssumeRolePolicyDocument") {
		return desired, nil
	}
