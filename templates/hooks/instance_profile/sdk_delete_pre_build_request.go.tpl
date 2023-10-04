
	// All roles need to be deleted before the instance profile
	// can be removed
	if r.ko.Spec.Role != nil {
		err = rm.detachRole(ctx, r)
		if err != nil {
		    return nil, err
		}
	}
