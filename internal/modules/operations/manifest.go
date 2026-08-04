package operations

import "github.com/korwlik23/go-api-starter/internal/modular"

func Manifest() modular.Manifest {
	return modular.Manifest{
		ID:                   "operations",
		Tier:                 modular.TierCore,
		Dependencies:         []string{"authorization"},
		RequiredCapabilities: []string{"authorization.evaluate", "database.sql"},
		ProvidedCapabilities: []string{"operations.modules"},
		Permissions:          Permissions(),
		Routes: []modular.Route{{
			Method:     "GET",
			Path:       "/api/v1/operations/modules",
			Permission: PermissionFoundationReadSystem,
		}, {
			Method: "GET", Path: "/api/v1/operations/jobs", Permission: PermissionJobsReadSystem,
		}, {
			Method: "POST", Path: "/api/v1/operations/jobs", Permission: PermissionJobsManageSystem,
		}, {
			Method: "POST", Path: "/api/v1/operations/jobs/:job_id/retry", Permission: PermissionJobsManageSystem,
		}},
		Migrations: modular.SQLMigrations(),
		Health: []modular.HealthContributor{
			{ID: "operations.modules", Critical: true},
		},
	}
}
