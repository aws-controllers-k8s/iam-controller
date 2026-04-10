	var policyARNs []*string
	if resp.AttachedPolicies != nil {
		for _, p := range resp.AttachedPolicies {
			if p.PolicyArn != nil {
				policyARNs = append(policyARNs, p.PolicyArn)
			}
		}
	}
	ko.Spec.PolicyARNs = policyARNs
