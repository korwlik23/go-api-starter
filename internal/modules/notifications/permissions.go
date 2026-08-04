package notifications

import "github.com/korwlik23/go-api-starter/internal/modular"

const (
	PermissionReadOwn   = "notifications.inbox.read.own"
	PermissionManageOwn = "notifications.inbox.manage.own"
)

func Permissions() []modular.Permission {
	return []modular.Permission{
		{Key: PermissionReadOwn, Scope: modular.ScopeOwn, Tier: modular.TierCore, Delegable: false},
		{Key: PermissionManageOwn, Scope: modular.ScopeOwn, Tier: modular.TierCore, Delegable: false},
	}
}
