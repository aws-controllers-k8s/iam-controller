
	// All roles need to be deleted before the instance profile
	// can be removed
	if r.ko.Spec.Role != nil {
		if err = rm.detachRole(ctx, r); err != nil {
		    return nil, err
		}
	}
