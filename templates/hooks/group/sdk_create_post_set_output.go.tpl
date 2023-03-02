    // This causes a requeue and policies/tags will be synced on the next
    // reconciliation loop
    ackcondition.SetSynced(&resource{ko}, corev1.ConditionFalse, nil, nil)
