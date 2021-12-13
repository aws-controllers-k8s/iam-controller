    // A Policy doesn't have an update code path. Instead, users can create a
    // new PolicyVersion and set that new version to be the default version of
    // a Policy. Until we implement this custom update code path, we will just
    // set the ResourceSynced condition to True here after a successful
    // creation (of the first, and as of now, only supported) version.
    ackcondition.SetSynced(&resource{ko}, corev1.ConditionTrue, nil, nil)
