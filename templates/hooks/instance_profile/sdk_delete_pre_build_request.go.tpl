
	// All roles need to be deleted before the instance profile
	// can be removed
	if r.ko.Spec.Role != nil {
		rm.detachRole(ctx, r)
	}
