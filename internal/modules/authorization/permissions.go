package authorization

import "github.com/korwlik23/go-api-starter/internal/modular"

const (
	PermissionEffectiveReadOwn          = "authorization.permissions.read.own"
	PermissionRolesReadSystem           = "authorization.roles.read.system"
	PermissionRolesManageSystem         = "authorization.roles.manage.system"
	PermissionRolePermissionsReadSystem = "authorization.role_permissions.read.system"
	PermissionRolePermissionGrant       = "authorization.role_permissions.grant.system"
	PermissionRoleAssignmentsReadSystem = "authorization.role_assignments.read.system"
	PermissionRoleAssignmentAssign      = "authorization.role_assignments.assign.system"
	PermissionRoleAssignmentRevoke      = "authorization.role_assignments.revoke.system"
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
			Key:       PermissionRolesReadSystem,
			Scope:     modular.ScopeSystem,
			Tier:      modular.TierCore,
			Delegable: false,
		},
		{
			Key:       PermissionRolesManageSystem,
			Scope:     modular.ScopeSystem,
			Tier:      modular.TierCore,
			Delegable: false,
		},
		{
			Key:       PermissionRolePermissionsReadSystem,
			Scope:     modular.ScopeSystem,
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
			Key:       PermissionRoleAssignmentsReadSystem,
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
