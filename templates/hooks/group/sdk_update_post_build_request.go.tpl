    // The code-generator expect the SDK field and the CR field to have
    // exact matching names. Which is not the case for `Path` (CR field)
    // and `NewPath` (SDK field). It is currently possible to set a CR
    // field from a custom field but the other way around. For now we
    // set those manually and wait for a better solution.
    // 
    // TODO(A-Hilaly): remove once aws-controllers-k8s/community#1532
    // or better idea is implemented
    if desired.ko.Spec.Path != nil {
        input.NewPath = desired.ko.Spec.Path
    }