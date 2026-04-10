	inlinePolicyMap := make(map[string]*string)
	if resp.PolicyNames != nil {
		for _, name := range resp.PolicyNames {
			// GetRolePolicy would be needed to retrieve the document for each
			// policy name. For now, store the name with a nil document.
			inlinePolicyMap[name] = nil
		}
	}
	ko.Spec.InlinePolicyMap = inlinePolicyMap
