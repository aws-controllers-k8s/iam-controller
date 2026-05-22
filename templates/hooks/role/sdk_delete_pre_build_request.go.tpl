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
	// Remove the role from all instance profiles it is attached to.
	// This handles cases where external systems (e.g. EKS Auto Mode) have
	// attached the role to instance profiles not managed by ACK.
	if err := rm.removeFromInstanceProfiles(ctx, r); err != nil {
		return nil, err
	}
