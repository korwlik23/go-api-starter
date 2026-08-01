package audit

import "github.com/korwlik23/go-api-starter/internal/modular"

func Manifest() modular.Manifest {
	return modular.Manifest{
		ID:                   "audit",
		Tier:                 modular.TierCore,
		Dependencies:         []string{"accounts", "identity"},
		RequiredCapabilities: []string{"accounts.resolve", "database.sql", "identity.sessions"},
		ProvidedCapabilities: []string{"audit.append"},
		Routes: []modular.Route{{
			Method: "GET", Path: "/api/v1/audit/events",
			Permission: PermissionEventsReadSystem,
		}},
		Permissions: Permissions(),
		Migrations:  modular.SQLMigrations(),
	}
}
