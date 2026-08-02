package navigation

import "github.com/korwlik23/go-api-starter/internal/modular"

func Manifest() modular.Manifest {
	return modular.Manifest{
		ID: "navigation", Tier: modular.TierDefault, Disableable: true,
		Dependencies: []string{"accounts", "authorization", "localization", "audit"},
		RequiredCapabilities: []string{
			"accounts.resolve", "authorization.evaluate", "audit.append", "database.sql", "localization.catalog",
		},
		ProvidedCapabilities: []string{"navigation.menus"},
		Permissions:          Permissions(),
		Routes: []modular.Route{
			{Method: "GET", Path: "/api/v1/navigation/menus", Permission: PermissionMenusReadOwn},
			{Method: "POST", Path: "/api/v1/navigation/menus", Permission: PermissionMenusWriteOwn},
			{Method: "GET", Path: "/api/v1/navigation/menus/:menu_id", Permission: PermissionMenusReadOwn},
			{Method: "PATCH", Path: "/api/v1/navigation/menus/:menu_id", Permission: PermissionMenusWriteOwn},
			{Method: "DELETE", Path: "/api/v1/navigation/menus/:menu_id", Permission: PermissionMenusWriteOwn},
			{Method: "POST", Path: "/api/v1/navigation/menus/:menu_id/reorder", Permission: PermissionMenusWriteOwn},
		},
		Migrations: modular.SQLMigrations(),
		Health:     []modular.HealthContributor{{ID: "navigation.menus", Critical: false}},
	}
}
