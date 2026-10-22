	if name, ok := fields["name"]; ok && fields["arn"] == "" {
		r.ko.Spec.Name = &name
		if path, ok := fields["path"]; ok {
			r.ko.Spec.Path = &path
		}
		return nil
	}
