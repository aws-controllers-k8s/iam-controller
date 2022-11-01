    if delta.DifferentAt("Spec.Policies") {
        if err := rm.syncPolicies(ctx, &resource{ko}); err != nil {
            return nil, err
        }
    }
    if delta.DifferentAt("Spec.Tags") {
        if err := rm.syncTags(ctx, &resource{ko}); err != nil {
            return nil, err
        }
    }
