	if ko.Spec.AssumeRolePolicyDocument != nil {
		if doc, err := decodeDocument(*ko.Spec.AssumeRolePolicyDocument); err != nil {
			return nil, err
		} else {
			// Normalize the JSON through the IAM policy library so that
			// single-element arrays (e.g. "Service": "ec2.amazonaws.com")
			// are expanded back to arrays, matching the user-provided format.
			var p awsiampolicy.Policy
			if err := json.Unmarshal([]byte(doc), &p); err == nil {
				if normalized, err := json.Marshal(p); err == nil {
					s := string(normalized)
					ko.Spec.AssumeRolePolicyDocument = &s
				} else {
					ko.Spec.AssumeRolePolicyDocument = &doc
				}
			} else {
				ko.Spec.AssumeRolePolicyDocument = &doc
			}
		}
	}
