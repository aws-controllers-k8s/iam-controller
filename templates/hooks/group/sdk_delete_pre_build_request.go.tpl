	// This deletes all associated managed and inline policies from the user
	groupCpy := r.ko.DeepCopy()
	groupCpy.Spec.Policies = nil
	if err := rm.syncManagedPolicies(ctx, &resource{ko: groupCpy}, r); err != nil {
		return nil, err
	}
	groupCpy.Spec.InlinePolicies = map[string]*string{}
	if err := rm.syncInlinePolicies(ctx, &resource{ko: groupCpy}, r); err != nil {
		return nil, err
	}
