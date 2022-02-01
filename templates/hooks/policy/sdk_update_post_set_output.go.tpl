    if err := rm.syncTags(ctx, &resource{ko}); err != nil {
        return nil, err
    }
