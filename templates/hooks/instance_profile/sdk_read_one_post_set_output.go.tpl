
	// Get the existing role associated with the instance profile. If the profile
	// has no role assigned, this field should be `nil`. This value is later
	// compared with the new desired value to ensure they are in sync.
	ko.Spec.Role = nil
	attachedRoles := resp.InstanceProfile.Roles
	if len(attachedRoles) > 0 {
		ko.Spec.Role = attachedRoles[0].RoleName
	}
