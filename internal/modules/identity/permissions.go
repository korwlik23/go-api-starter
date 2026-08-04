package identity

import "github.com/korwlik23/go-api-starter/internal/modular"

const (
	PermissionUsersReadSystem   = "identity.users.read.system"
	PermissionUsersManageSystem = "identity.users.manage.system"
	PermissionProfileReadOwn    = "identity.profile.read.own"
	PermissionProfileManageOwn  = "identity.profile.manage.own"
	PermissionPasswordManageOwn = "identity.password.manage.own"
)

func Permissions() []modular.Permission {
	return []modular.Permission{
		{Key: PermissionUsersReadSystem, Scope: modular.ScopeSystem, Tier: modular.TierCore, Delegable: false},
		{Key: PermissionUsersManageSystem, Scope: modular.ScopeSystem, Tier: modular.TierCore, Delegable: false},
		{Key: PermissionProfileReadOwn, Scope: modular.ScopeOwn, Tier: modular.TierCore, Delegable: false},
		{Key: PermissionProfileManageOwn, Scope: modular.ScopeOwn, Tier: modular.TierCore, Delegable: false},
		{Key: PermissionPasswordManageOwn, Scope: modular.ScopeOwn, Tier: modular.TierCore, Delegable: false},
	}
}
