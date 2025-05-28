    tmp, ok := identifier.AdditionalKeys["roleName"]
	if !ok {
		return ackerrors.NewTerminalError(fmt.Errorf("required field missing: roleName"))
	}
	r.ko.Spec.AWSServiceName = &tmp
