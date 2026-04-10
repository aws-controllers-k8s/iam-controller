	var tags []*svcapitypes.Tag
	if resp.Tags != nil {
		for _, t := range resp.Tags {
			tag := &svcapitypes.Tag{}
			if t.Key != nil {
				tag.Key = t.Key
			}
			if t.Value != nil {
				tag.Value = t.Value
			}
			tags = append(tags, tag)
		}
	}
	ko.Spec.Tags = tags
