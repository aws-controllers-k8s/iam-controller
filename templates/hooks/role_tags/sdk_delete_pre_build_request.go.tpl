	// Set TagKeys from the Key field for the UntagRole API call
	if r.ko.Spec.Key != nil {
		r.ko.Spec.TagKeys = []*string{r.ko.Spec.Key}
	}
