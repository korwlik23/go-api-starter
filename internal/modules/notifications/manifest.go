package notifications

import "github.com/korwlik23/go-api-starter/internal/modular"

func Manifest() modular.Manifest {
	return modular.Manifest{
		ID: "notifications", Tier: modular.TierCore,
		Dependencies:         []string{"identity", "authorization"},
		RequiredCapabilities: []string{"identity.sessions", "authorization.evaluate", "database.sql"},
		ProvidedCapabilities: []string{"notifications.in_app"},
		Permissions:          Permissions(),
		Routes: []modular.Route{
			{Method: "GET", Path: "/api/v1/notifications", Permission: PermissionReadOwn},
			{Method: "PATCH", Path: "/api/v1/notifications/:notification_id/read", Permission: PermissionManageOwn},
			{Method: "POST", Path: "/api/v1/notifications/read-all", Permission: PermissionManageOwn},
		},
		Migrations: modular.SQLMigrations(),
		Health:     []modular.HealthContributor{{ID: "notifications.in_app", Critical: false}},
	}
}
