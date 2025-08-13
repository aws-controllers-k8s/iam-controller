	// If the Tags field has changed, sync the tags
	if delta.DifferentAt("Spec.Tags") {
		err := rm.syncTags(
			ctx,
			latest,
			desired,
		)
		if err != nil {
			return nil, err
		}
	}
	
	// If the only difference is the Tags field, we've already handled it
	// so we can return the desired object and skip the actual Update call
	if !delta.DifferentExcept("Spec.Tags") {
		return desired, nil
	}