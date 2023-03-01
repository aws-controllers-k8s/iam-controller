	// This deletes all associated managed and inline policies from the role
	roleCpy := r.ko.DeepCopy()
	roleCpy.Spec.Policies = nil
	if err := rm.syncManagedPolicies(ctx, &resource{ko: roleCpy}, r); err != nil {
		return nil, err
	}
	roleCpy.Spec.InlinePolicies = map[string]*string{}
	if err := rm.syncInlinePolicies(ctx, &resource{ko: roleCpy}, r); err != nil {
		return nil, err
	}
