package application

import (
	"strings"

	authorizationmodule "github.com/korwlik23/go-api-starter/internal/modules/authorization"
)

const (
	PermissionRoleAssignmentAssign = authorizationmodule.PermissionRoleAssignmentAssign
	PermissionRoleAssignmentRevoke = authorizationmodule.PermissionRoleAssignmentRevoke
)

type RoleAssignmentPolicyInput struct {
	GrantorPermissions map[string]bool
	Scope              string
	RoleOwner          RoleOwner
	RoleAccountID      string
	TargetAccountID    string
	TargetActive       bool
	ModuleEnabled      bool
	AlreadyAssigned    bool
}

func ValidateRoleAssignment(input RoleAssignmentPolicyInput) error {
	if input.GrantorPermissions == nil ||
		!input.GrantorPermissions[PermissionRoleAssignmentAssign] ||
		!input.TargetActive ||
		!input.ModuleEnabled ||
		input.AlreadyAssigned {
		return ErrRoleAssignmentDenied
	}
	switch input.Scope {
	case AssignmentScopeSystem:
		if input.RoleOwner != RoleOwnerSystem ||
			strings.TrimSpace(input.TargetAccountID) != "" {
			return ErrRoleAssignmentDenied
		}
	case AssignmentScopeAccount:
		if input.RoleOwner != RoleOwnerAccount ||
			strings.TrimSpace(input.RoleAccountID) == "" ||
			input.RoleAccountID != input.TargetAccountID {
			return ErrRoleAssignmentDenied
		}
	default:
		return ErrRoleAssignmentDenied
	}
	return nil
}
