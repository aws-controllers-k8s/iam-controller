    if ko.Spec.AssumeRolePolicyDocument != nil {
        if doc, err := decodeDocument(*ko.Spec.AssumeRolePolicyDocument); err != nil {
            return nil, err
        } else {
            ko.Spec.AssumeRolePolicyDocument = &doc
        }
    }
    return &resource{ko}, ackrequeue.Needed(fmt.Errorf("role created, requeuing to trigger updates"))
