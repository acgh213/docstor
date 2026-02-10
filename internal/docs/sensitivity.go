package docs

// CanAccess checks if a role can access a document's sensitivity level.
// MVP policy (claude.md §3):
//   - public-internal: all roles
//   - restricted/confidential: admin + editor only
func CanAccess(role string, sensitivity Sensitivity) bool {
	switch sensitivity {
	case SensitivityPublic, "":
		return true
	case SensitivityRestricted, SensitivityConfidential:
		return role == "admin" || role == "editor"
	default:
		// Unknown sensitivity — deny by default
		return false
	}
}
