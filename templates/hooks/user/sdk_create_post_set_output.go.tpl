    if err := rm.syncPolicies(ctx, &resource{ko}); err != nil {
        return nil, err
    }
