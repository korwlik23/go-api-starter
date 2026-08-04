package settings

import "github.com/korwlik23/go-api-starter/internal/modular"

func Manifest() modular.Manifest {
	return modular.Manifest{
		ID: "settings", Tier: modular.TierCore,
		Dependencies:         []string{"authorization"},
		RequiredCapabilities: []string{"authorization.evaluate", "database.sql"},
		ProvidedCapabilities: []string{"settings.global"},
		Permissions:          Permissions(),
		Routes: []modular.Route{
			{Method: "GET", Path: "/api/v1/settings", Permission: PermissionReadSystem},
			{Method: "PUT", Path: "/api/v1/settings/:key", Permission: PermissionManageSystem},
		},
		Migrations: modular.SQLMigrations(),
		Health:     []modular.HealthContributor{{ID: "settings.global", Critical: false}},
	}
}
