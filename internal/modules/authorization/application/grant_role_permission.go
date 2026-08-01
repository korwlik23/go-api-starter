package application

import (
	"errors"
	"strings"

	authorizationmodule "github.com/korwlik23/go-api-starter/internal/modules/authorization"
	authorizationdomain "github.com/korwlik23/go-api-starter/internal/modules/authorization/domain"
)

var ErrGrantDenied = errors.New("role permission grant denied")

const PermissionRolePermissionGrant = authorizationmodule.PermissionRolePermissionGrant

type RoleOwner string

const (
	RoleOwnerSystem  RoleOwner = RoleOwner(authorizationdomain.RoleOwnerSystem)
	RoleOwnerAccount RoleOwner = RoleOwner(authorizationdomain.RoleOwnerAccount)
)

type PermissionScope string

const (
	PermissionScopeOwn    PermissionScope = PermissionScope(authorizationdomain.PermissionScopeOwn)
	PermissionScopeAny    PermissionScope = PermissionScope(authorizationdomain.PermissionScopeAny)
	PermissionScopeSystem PermissionScope = PermissionScope(authorizationdomain.PermissionScopeSystem)
)

type GrantRolePermissionInput struct {
	GrantorPermissions  map[string]bool
	RoleOwner           RoleOwner
	TargetPermissionKey string
	TargetScope         PermissionScope
	TargetActive        bool
	TargetDelegable     bool
	ModuleEnabled       bool
}

// ValidateGrantRolePermission is the fail-closed policy used immediately before
// a transactional role-permission insert. Permission implication is intentionally
// absent: a grantor must hold the exact target permission key.
func ValidateGrantRolePermission(input GrantRolePermissionInput) error {
	if input.GrantorPermissions == nil ||
		strings.TrimSpace(input.TargetPermissionKey) == "" ||
		!input.TargetActive ||
		!input.TargetDelegable ||
		!input.ModuleEnabled ||
		!input.GrantorPermissions[PermissionRolePermissionGrant] ||
		!input.GrantorPermissions[input.TargetPermissionKey] {
		return ErrGrantDenied
	}
	key, err := authorizationdomain.ParsePermissionKey(input.TargetPermissionKey)
	if err != nil || PermissionScope(key.Scope) != input.TargetScope {
		return ErrGrantDenied
	}
	if input.RoleOwner == RoleOwnerSystem && input.TargetScope != PermissionScopeSystem {
		return ErrGrantDenied
	}
	if input.RoleOwner == RoleOwnerAccount && input.TargetScope == PermissionScopeSystem {
		return ErrGrantDenied
	}
	if input.RoleOwner != RoleOwnerSystem && input.RoleOwner != RoleOwnerAccount {
		return ErrGrantDenied
	}
	return nil
}
