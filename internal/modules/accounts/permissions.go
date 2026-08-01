package accounts

import "github.com/korwlik23/go-api-starter/internal/modular"

const PermissionCurrentReadOwn = "accounts.current.read.own"

const (
	PermissionMembershipsReadSystem   = "accounts.memberships.read.system"
	PermissionMembershipsManageSystem = "accounts.memberships.manage.system"
)

func Permissions() []modular.Permission {
	return []modular.Permission{{
		Key:       PermissionCurrentReadOwn,
		Scope:     modular.ScopeOwn,
		Tier:      modular.TierCore,
		Delegable: false,
	}, {
		Key: PermissionMembershipsReadSystem, Scope: modular.ScopeSystem,
		Tier: modular.TierCore, Delegable: false,
	}, {
		Key: PermissionMembershipsManageSystem, Scope: modular.ScopeSystem,
		Tier: modular.TierCore, Delegable: false,
	}}
}
