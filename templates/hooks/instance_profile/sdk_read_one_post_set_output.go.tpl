
	// Get the existing role associated with the instance profile if
	// any. This value needs to later be compared with the user
	// specified value to make sure they are in sync
	ko.Spec.Role = nil
	attachedRoles := resp.InstanceProfile.Roles
	if len(attachedRoles) > 0 {
		ko.Spec.Role = attachedRoles[0].RoleName
	}
