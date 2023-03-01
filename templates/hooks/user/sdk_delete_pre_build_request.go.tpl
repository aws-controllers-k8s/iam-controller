	// This deletes all associated managed and inline policies from the user
	userCpy := r.ko.DeepCopy()
	userCpy.Spec.Policies = nil
	if err := rm.syncManagedPolicies(ctx, &resource{ko: userCpy}, r); err != nil {
		return nil, err
	}
	userCpy.Spec.InlinePolicies = map[string]*string{}
	if err := rm.syncInlinePolicies(ctx, &resource{ko: userCpy}, r); err != nil {
		return nil, err
	}
