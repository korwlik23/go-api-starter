package identity

import "github.com/korwlik23/go-api-starter/internal/modular"

const (
	PermissionUsersReadSystem   = "identity.users.read.system"
	PermissionUsersManageSystem = "identity.users.manage.system"
)

func Permissions() []modular.Permission {
	return []modular.Permission{
		{Key: PermissionUsersReadSystem, Scope: modular.ScopeSystem, Tier: modular.TierCore, Delegable: false},
		{Key: PermissionUsersManageSystem, Scope: modular.ScopeSystem, Tier: modular.TierCore, Delegable: false},
	}
}
