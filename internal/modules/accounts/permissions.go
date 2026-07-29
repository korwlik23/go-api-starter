package accounts

import "github.com/korwlik23/go-api-starter/internal/modular"

const PermissionCurrentReadOwn = "accounts.current.read.own"

func Permissions() []modular.Permission {
	return []modular.Permission{{
		Key:       PermissionCurrentReadOwn,
		Scope:     modular.ScopeOwn,
		Tier:      modular.TierCore,
		Delegable: false,
	}}
}
