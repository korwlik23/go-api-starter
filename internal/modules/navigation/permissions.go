package navigation

import "github.com/korwlik23/go-api-starter/internal/modular"

const (
	PermissionMenusReadOwn    = "navigation.menus.read.own"
	PermissionMenusWriteOwn   = "navigation.menus.write.own"
	PermissionMenusPublishAny = "navigation.menus.publish.any"
)

func Permissions() []modular.Permission {
	return []modular.Permission{
		{Key: PermissionMenusReadOwn, Scope: modular.ScopeOwn, Tier: modular.TierDefault, Delegable: false},
		{Key: PermissionMenusWriteOwn, Scope: modular.ScopeOwn, Tier: modular.TierDefault, Delegable: false},
		{Key: PermissionMenusPublishAny, Scope: modular.ScopeAny, Tier: modular.TierDefault, Delegable: false},
	}
}
