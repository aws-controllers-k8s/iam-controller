    // A managed Policy whose ARN never reached the status: derive the ARN so
    // that GetPolicy finds it. See orphanedPolicyARN.
    if orphanARN := rm.orphanedPolicyARN(r); orphanARN != nil {
        ko := r.ko.DeepCopy()
        if ko.Status.ACKResourceMetadata == nil {
            ko.Status.ACKResourceMetadata = &ackv1alpha1.ResourceMetadata{}
        }
        ko.Status.ACKResourceMetadata.ARN = orphanARN
        r = &resource{ko}
    }
