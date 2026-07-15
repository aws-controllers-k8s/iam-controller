	// This deletes all associated managed and inline policies from the role
	roleCpy := r.ko.DeepCopy()
	// Clear the additive policy-reconciliation annotation on the copy so that
	// syncManagedPolicies performs a full detach of every attached managed
	// policy. IAM requires a role to have no attached managed policies before
	// DeleteRole succeeds, so teardown must always be authoritative.
	if roleCpy.ObjectMeta.Annotations != nil {
		delete(roleCpy.ObjectMeta.Annotations, policyReconcileModeAnnotation)
	}
	roleCpy.Spec.Policies = nil
	if err := rm.syncManagedPolicies(ctx, &resource{ko: roleCpy}, r); err != nil {
		return nil, err
	}
	roleCpy.Spec.InlinePolicies = map[string]*string{}
	if err := rm.syncInlinePolicies(ctx, &resource{ko: roleCpy}, r); err != nil {
		return nil, err
	}
