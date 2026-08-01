package application

import "errors"

var ErrLastManager = errors.New("last manager invariant would be violated")

// ValidateLastManagerRevoke receives effective permission sets after the target
// assignment is removed. A manager is defined only by the required permission
// keys; role names and weaker scopes never satisfy the invariant.
func ValidateLastManagerRevoke(
	targetAfter map[string]bool,
	otherPrincipals []map[string]bool,
	required []string,
) error {
	if hasAllPermissions(targetAfter, required) {
		return nil
	}
	for _, principal := range otherPrincipals {
		if hasAllPermissions(principal, required) {
			return nil
		}
	}
	return ErrLastManager
}

func hasAllPermissions(grants map[string]bool, required []string) bool {
	if len(required) == 0 || grants == nil {
		return false
	}
	for _, key := range required {
		if !grants[key] {
			return false
		}
	}
	return true
}
