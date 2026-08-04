package localization

import "github.com/korwlik23/go-api-starter/internal/modular"

func Manifest() modular.Manifest {
	return modular.Manifest{
		ID:                   "localization",
		Tier:                 modular.TierCore,
		Dependencies:         []string{"identity"},
		RequiredCapabilities: []string{"database.sql", "identity.sessions"},
		ProvidedCapabilities: []string{"localization.catalog"},
		Permissions:          Permissions(),
		Routes: []modular.Route{
			{Method: "GET", Path: "/api/v1/locales"},
			{Method: "POST", Path: "/api/v1/locales", Permission: PermissionLocalesManageSystem},
			{Method: "PATCH", Path: "/api/v1/locales/:locale_id", Permission: PermissionLocalesManageSystem},
			{Method: "PATCH", Path: "/api/v1/locales/:locale_id/status", Permission: PermissionLocalesManageSystem},
			{Method: "PUT", Path: "/api/v1/locales/:locale_id/default", Permission: PermissionLocalesManageSystem},
			{Method: "GET", Path: "/api/v1/locales/preferences"},
			{Method: "PUT", Path: "/api/v1/locales/preferences"},
			{Method: "GET", Path: "/api/v1/localization/catalog"},
			{Method: "PUT", Path: "/api/v1/localization/catalog", Permission: PermissionTranslationsUpdateSystem},
			{Method: "GET", Path: "/api/v1/localization/catalog/export", Permission: PermissionTranslationsReadSystem},
			{Method: "POST", Path: "/api/v1/localization/catalog/import", Permission: PermissionTranslationsUpdateSystem},
		},
		Migrations: modular.SQLMigrations(),
		Health: []modular.HealthContributor{
			{ID: "localization.catalog", Critical: true},
		},
	}
}
