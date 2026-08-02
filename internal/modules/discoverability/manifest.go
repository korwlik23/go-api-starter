package discoverability

import "github.com/korwlik23/go-api-starter/internal/modular"

func Manifest() modular.Manifest {
	return modular.Manifest{
		ID: "discoverability", Tier: modular.TierDefault, Disableable: true,
		Dependencies:         []string{"accounts", "authorization", "localization", "audit"},
		RequiredCapabilities: []string{"accounts.resolve", "authorization.evaluate", "audit.append", "database.sql", "localization.catalog"},
		ProvidedCapabilities: []string{"discoverability.redirects", "discoverability.seo"},
		Permissions:          Permissions(),
		Routes: []modular.Route{
			{Method: "GET", Path: "/api/v1/discoverability/redirects", Permission: PermissionRedirectsReadOwn},
			{Method: "POST", Path: "/api/v1/discoverability/redirects", Permission: PermissionRedirectsWriteOwn},
			{Method: "GET", Path: "/api/v1/discoverability/redirects/:redirect_id", Permission: PermissionRedirectsReadOwn},
			{Method: "PATCH", Path: "/api/v1/discoverability/redirects/:redirect_id", Permission: PermissionRedirectsWriteOwn},
			{Method: "DELETE", Path: "/api/v1/discoverability/redirects/:redirect_id", Permission: PermissionRedirectsWriteOwn},
			{Method: "GET", Path: "/api/v1/discoverability/seo", Permission: PermissionSEOReadOwn},
			{Method: "PATCH", Path: "/api/v1/discoverability/seo", Permission: PermissionSEOWriteOwn},
		},
		Migrations: modular.SQLMigrations(),
		Health:     []modular.HealthContributor{{ID: "discoverability.redirects", Critical: false}},
	}
}
