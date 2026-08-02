package app

import (
	"testing"

	"github.com/gin-gonic/gin"
	discoverabilityhttp "github.com/korwlik23/go-api-starter/internal/modules/discoverability/transport/http"
	mediahttp "github.com/korwlik23/go-api-starter/internal/modules/media/transport/http"
	navigationhttp "github.com/korwlik23/go-api-starter/internal/modules/navigation/transport/http"
	publishinghttp "github.com/korwlik23/go-api-starter/internal/modules/publishing/transport/http"
	"github.com/stretchr/testify/require"
)

func TestM3F6CMSRoutesComposeProtectedAdminAndPublicSurfaces(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	noOp := func(ctx *gin.Context) { ctx.Next() }
	guards := cmsRouteGuards{
		pagesRead: noOp, pagesWrite: noOp, pagesReview: noOp,
		pagesPublish: noOp, revisionsRead: noOp, revisionsRollback: noOp,
		mediaUploadsWrite: noOp, mediaAssetsRead: noOp, mediaAssetsWrite: noOp,
		menusRead: noOp, menusWrite: noOp, redirectsRead: noOp,
		redirectsWrite: noOp, seoRead: noOp, seoWrite: noOp,
		auditsRead: noOp, auditsRun: noOp,
	}
	RegisterCMSRoutes(router.Group("/api/v1"), cmsRouteRegistration{
		publishing: publishinghttp.RouteHandlers{
			Content: &publishinghttp.ContentHandler{}, Revisions: &publishinghttp.RevisionHandler{},
			Workflow: &publishinghttp.WorkflowHandler{}, Schedules: &publishinghttp.ScheduleHandler{},
			Preview: &publishinghttp.PreviewHandler{},
		},
		public:    &publishinghttp.PublicHandler{},
		preview:   &publishinghttp.PreviewHandler{},
		uploads:   &mediahttp.UploadHandler{},
		assets:    &mediahttp.AssetHandler{},
		menus:     &navigationhttp.MenuHandler{},
		redirects: &discoverabilityhttp.RedirectHandler{},
		settings:  &discoverabilityhttp.SettingsHandler{},
		audits:    &discoverabilityhttp.AuditHandler{},
		guards:    guards,
	}, identityRuntime{authenticate: noOp, csrf: noOp})

	require.ElementsMatch(t, []string{
		"POST /api/v1/publishing/pages", "PATCH /api/v1/publishing/pages/:page_id",
		"DELETE /api/v1/publishing/pages/:page_id", "POST /api/v1/publishing/posts",
		"PATCH /api/v1/publishing/posts/:post_id", "DELETE /api/v1/publishing/posts/:post_id",
		"GET /api/v1/publishing/content/:content_id/revisions",
		"POST /api/v1/publishing/content/:content_id/revisions/:revision_id/rollback",
		"POST /api/v1/publishing/content/:content_id/workflow", "GET /api/v1/publishing/schedules",
		"POST /api/v1/publishing/schedules", "DELETE /api/v1/publishing/schedules/:schedule_id",
		"POST /api/v1/publishing/content/:content_id/preview",
		"GET /api/v1/public/content/:locale/*path", "GET /api/v1/public/posts/:locale",
		"GET /api/v1/public/posts/:locale/:slug", "GET /api/v1/public/menus/:locale/:location",
		"POST /api/v1/public/previews/exchange",
		"POST /api/v1/media/uploads", "POST /api/v1/media/uploads/:upload_id/finalize",
		"GET /api/v1/media/assets", "DELETE /api/v1/media/assets/:asset_id",
		"GET /api/v1/navigation/menus", "POST /api/v1/navigation/menus",
		"GET /api/v1/navigation/menus/:menu_id", "PATCH /api/v1/navigation/menus/:menu_id",
		"DELETE /api/v1/navigation/menus/:menu_id", "POST /api/v1/navigation/menus/:menu_id/reorder",
		"GET /api/v1/discoverability/redirects", "POST /api/v1/discoverability/redirects",
		"GET /api/v1/discoverability/redirects/:redirect_id",
		"PATCH /api/v1/discoverability/redirects/:redirect_id",
		"DELETE /api/v1/discoverability/redirects/:redirect_id",
		"GET /api/v1/discoverability/seo", "PATCH /api/v1/discoverability/seo",
		"GET /api/v1/discoverability/audits", "POST /api/v1/discoverability/audits",
		"GET /api/v1/discoverability/audits/:audit_id",
	}, routeKeys(router))
}

func TestM3F6DisabledCMSModulesDoNotRegisterDeadRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	RegisterCMSRoutes(router.Group("/api/v1"), cmsRouteRegistration{}, identityRuntime{})
	require.Empty(t, router.Routes())
}
