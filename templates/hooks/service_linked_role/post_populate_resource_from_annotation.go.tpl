    tmp, ok := fields["roleName"]
	if !ok {
		return ackerrors.NewTerminalError(fmt.Errorf("required field missing: roleName"))
	}
	r.ko.Status.RoleName = &tmp
