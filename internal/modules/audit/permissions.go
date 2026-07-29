package audit

import "github.com/korwlik23/go-api-starter/internal/modular"

const PermissionEventsReadSystem = "audit.events.read.system"

func Permissions() []modular.Permission {
	return []modular.Permission{{
		Key:       PermissionEventsReadSystem,
		Scope:     modular.ScopeSystem,
		Tier:      modular.TierCore,
		Delegable: false,
	}}
}
