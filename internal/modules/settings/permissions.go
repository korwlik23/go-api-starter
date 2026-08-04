package settings

import "github.com/korwlik23/go-api-starter/internal/modular"

const (
	PermissionReadSystem   = "settings.global.read.system"
	PermissionManageSystem = "settings.global.manage.system"
)

func Permissions() []modular.Permission {
	return []modular.Permission{
		{Key: PermissionReadSystem, Scope: modular.ScopeSystem, Tier: modular.TierCore, Delegable: false},
		{Key: PermissionManageSystem, Scope: modular.ScopeSystem, Tier: modular.TierCore, Delegable: false},
	}
}
