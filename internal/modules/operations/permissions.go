package operations

import "github.com/korwlik23/go-api-starter/internal/modular"

const PermissionFoundationReadSystem = "operations.foundation.read.system"

func Permissions() []modular.Permission {
	return []modular.Permission{{
		Key:       PermissionFoundationReadSystem,
		Scope:     modular.ScopeSystem,
		Tier:      modular.TierCore,
		Delegable: false,
	}}
}
