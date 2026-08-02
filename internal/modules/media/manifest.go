package media

import "github.com/korwlik23/go-api-starter/internal/modular"

func Manifest() modular.Manifest {
	return modular.Manifest{
		ID:          "media",
		Tier:        modular.TierDefault,
		Disableable: true,
		Dependencies: []string{
			"accounts",
			"authorization",
			"audit",
		},
		RequiredCapabilities: []string{
			"accounts.resolve",
			"authorization.evaluate",
			"audit.append",
			"database.sql",
		},
		ProvidedCapabilities: []string{"media.assets"},
		Permissions:          Permissions(),
		Routes: []modular.Route{
			{Method: "POST", Path: "/api/v1/media/uploads", Permission: PermissionUploadsWriteOwn},
			{Method: "POST", Path: "/api/v1/media/uploads/:upload_id/finalize", Permission: PermissionUploadsWriteOwn},
			{Method: "GET", Path: "/api/v1/media/assets", Permission: PermissionAssetsReadOwn},
			{Method: "DELETE", Path: "/api/v1/media/assets/:asset_id", Permission: PermissionAssetsWriteOwn},
		},
		Migrations: modular.SQLMigrations(),
		Health:     []modular.HealthContributor{{ID: "media.assets", Critical: true}},
	}
}
