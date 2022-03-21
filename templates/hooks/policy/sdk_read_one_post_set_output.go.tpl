    // The PolicyDocument is not returned by GetPolicy. You need to call
    // GetPolicyVersion with the default version ID in order to retrieve it
    if ko.Status.DefaultVersionID != nil && ko.Status.ACKResourceMetadata != nil && ko.Status.ACKResourceMetadata.ARN != nil {
        policyARN := string(*ko.Status.ACKResourceMetadata.ARN)
        version := *ko.Status.DefaultVersionID
        if pv, err := rm.getPolicyVersion(ctx, policyARN, version); err != nil {
            return nil, err
        } else {
            ko.Spec.PolicyDocument = &pv.document
        }
    }
