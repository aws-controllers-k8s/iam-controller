	// This is to avoid the following error:
	//
	// DeleteConflict: This policy has more than one version. Before you delete a
	// policy, you must delete the policy's versions. The default version is
	// deleted with the policy.
	if err = rm.deleteNonDefaultPolicyVersions(ctx, r); err != nil {
		return r, err
	}
