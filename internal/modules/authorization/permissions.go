package authorization

import "github.com/korwlik23/go-api-starter/internal/modular"

const (
	PermissionEffectiveReadOwn     = "authorization.permissions.read.own"
	PermissionRolePermissionGrant  = "authorization.role_permissions.grant.system"
	PermissionRoleAssignmentAssign = "authorization.role_assignments.assign.system"
	PermissionRoleAssignmentRevoke = "authorization.role_assignments.revoke.system"
)

func Permissions() []modular.Permission {
	return []modular.Permission{
		{
			Key:       PermissionEffectiveReadOwn,
			Scope:     modular.ScopeOwn,
			Tier:      modular.TierCore,
			Delegable: false,
		},
		{
			Key:       PermissionRolePermissionGrant,
			Scope:     modular.ScopeSystem,
			Tier:      modular.TierCore,
			Delegable: false,
		},
		{
			Key:       PermissionRoleAssignmentAssign,
			Scope:     modular.ScopeSystem,
			Tier:      modular.TierCore,
			Delegable: false,
		},
		{
			Key:       PermissionRoleAssignmentRevoke,
			Scope:     modular.ScopeSystem,
			Tier:      modular.TierCore,
			Delegable: false,
		},
	}
}
