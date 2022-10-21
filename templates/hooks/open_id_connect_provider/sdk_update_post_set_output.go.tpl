    if err := rm.syncTags(ctx, &resource{ko}); err != nil {
        return nil, err
    }
    // There really isn't a status of an OIDC Provider... it either exists or doesn't. If
    // we get here, that means the update was successful and the desired state
    // of the OIDC Provider matches what we provided...
    ackcondition.SetSynced(&resource{ko}, corev1.ConditionTrue, nil, nil)
