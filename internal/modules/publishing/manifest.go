package publishing

import "github.com/korwlik23/go-api-starter/internal/modular"

func Manifest() modular.Manifest {
	return modular.Manifest{
		ID:          "publishing",
		Tier:        modular.TierDefault,
		Disableable: true,
		Dependencies: []string{
			"accounts",
			"authorization",
			"localization",
			"audit",
		},
		RequiredCapabilities: []string{
			"accounts.resolve",
			"authorization.evaluate",
			"audit.append",
			"database.sql",
			"localization.catalog",
		},
		ProvidedCapabilities: []string{"publishing.content"},
		Permissions:          Permissions(),
		Routes: []modular.Route{
			{
				Method:     "GET",
				Path:       "/api/v1/publishing/pages",
				Permission: PermissionPagesReadOwn,
			},
			{
				Method:     "POST",
				Path:       "/api/v1/publishing/pages",
				Permission: PermissionPagesWriteOwn,
			},
			{
				Method:     "GET",
				Path:       "/api/v1/publishing/pages/:page_id",
				Permission: PermissionPagesReadOwn,
			},
			{
				Method:     "PATCH",
				Path:       "/api/v1/publishing/pages/:page_id",
				Permission: PermissionPagesWriteOwn,
			},
			{
				Method:     "DELETE",
				Path:       "/api/v1/publishing/pages/:page_id",
				Permission: PermissionPagesWriteOwn,
			},
			{
				Method:     "GET",
				Path:       "/api/v1/publishing/posts",
				Permission: PermissionPagesReadOwn,
			},
			{
				Method:     "POST",
				Path:       "/api/v1/publishing/posts",
				Permission: PermissionPagesWriteOwn,
			},
			{
				Method:     "GET",
				Path:       "/api/v1/publishing/posts/:post_id",
				Permission: PermissionPagesReadOwn,
			},
			{
				Method:     "PATCH",
				Path:       "/api/v1/publishing/posts/:post_id",
				Permission: PermissionPagesWriteOwn,
			},
			{
				Method:     "DELETE",
				Path:       "/api/v1/publishing/posts/:post_id",
				Permission: PermissionPagesWriteOwn,
			},
			{
				Method:     "GET",
				Path:       "/api/v1/publishing/content/:content_id/revisions",
				Permission: PermissionRevisionsReadOwn,
			},
			{
				Method:     "POST",
				Path:       "/api/v1/publishing/content/:content_id/revisions/:revision_id/rollback",
				Permission: PermissionRevisionsRollbackOwn,
			},
			{
				Method:     "POST",
				Path:       "/api/v1/publishing/content/:content_id/workflow",
				Permission: PermissionPagesReviewAny,
			},
			{
				Method:     "GET",
				Path:       "/api/v1/publishing/schedules",
				Permission: PermissionPagesReadOwn,
			},
			{
				Method:     "POST",
				Path:       "/api/v1/publishing/schedules",
				Permission: PermissionPagesPublishAny,
			},
			{
				Method:     "GET",
				Path:       "/api/v1/publishing/categories",
				Permission: PermissionPagesReadOwn,
			},
			{
				Method:     "POST",
				Path:       "/api/v1/publishing/categories",
				Permission: PermissionPagesWriteOwn,
			},
			{
				Method:     "GET",
				Path:       "/api/v1/publishing/tags",
				Permission: PermissionPagesReadOwn,
			},
			{
				Method:     "POST",
				Path:       "/api/v1/publishing/tags",
				Permission: PermissionPagesWriteOwn,
			},
			{
				Method:     "POST",
				Path:       "/api/v1/publishing/content/:content_id/preview",
				Permission: PermissionPagesReadOwn,
			},
			{
				Method: "GET",
				Path:   "/api/v1/public/content/:locale/*path",
			},
			{
				Method: "POST",
				Path:   "/api/v1/public/previews/exchange",
			},
		},
		Jobs:       []modular.Job{{ID: "publishing.schedule.process"}},
		Migrations: modular.SQLMigrations(),
		Health: []modular.HealthContributor{
			{ID: "publishing.content", Critical: true},
		},
	}
}
