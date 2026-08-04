package operations

import "github.com/korwlik23/go-api-starter/internal/modular"

const (
	PermissionFoundationReadSystem = "operations.foundation.read.system"
	PermissionJobsReadSystem       = "operations.jobs.read.system"
	PermissionJobsManageSystem     = "operations.jobs.manage.system"
)

func Permissions() []modular.Permission {
	return []modular.Permission{{
		Key:       PermissionFoundationReadSystem,
		Scope:     modular.ScopeSystem,
		Tier:      modular.TierCore,
		Delegable: false,
	}, {
		Key: PermissionJobsReadSystem, Scope: modular.ScopeSystem,
		Tier: modular.TierCore, Delegable: false,
	}, {
		Key: PermissionJobsManageSystem, Scope: modular.ScopeSystem,
		Tier: modular.TierCore, Delegable: false,
	}}
}
