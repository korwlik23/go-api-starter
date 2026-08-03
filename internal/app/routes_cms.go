package app

import (
	"github.com/gin-gonic/gin"
	discoverabilityhttp "github.com/korwlik23/go-api-starter/internal/modules/discoverability/transport/http"
	mediahttp "github.com/korwlik23/go-api-starter/internal/modules/media/transport/http"
	navigationhttp "github.com/korwlik23/go-api-starter/internal/modules/navigation/transport/http"
	publishinghttp "github.com/korwlik23/go-api-starter/internal/modules/publishing/transport/http"
)

// cmsRouteRegistration is the runtime boundary for optional CMS modules. A
// nil handler means the module is disabled and therefore contributes no route.
// Permission guards are supplied by the application composition layer so this
// file remains a route-only concern.
type cmsRouteRegistration struct {
	publishing publishinghttp.RouteHandlers
	public     *publishinghttp.PublicHandler
	preview    *publishinghttp.PreviewHandler
	uploads    *mediahttp.UploadHandler
	assets     *mediahttp.AssetHandler
	menus      *navigationhttp.MenuHandler
	redirects  *discoverabilityhttp.RedirectHandler
	settings   *discoverabilityhttp.SettingsHandler
	audits     *discoverabilityhttp.AuditHandler
	guards     cmsRouteGuards
}

type cmsRouteGuards struct {
	pagesRead, pagesWrite, pagesReview, pagesPublish     gin.HandlerFunc
	revisionsRead, revisionsRollback                     gin.HandlerFunc
	mediaUploadsWrite, mediaAssetsRead, mediaAssetsWrite gin.HandlerFunc
	menusRead, menusWrite                                gin.HandlerFunc
	redirectsRead, redirectsWrite, seoRead, seoWrite     gin.HandlerFunc
	auditsRead, auditsRun                                gin.HandlerFunc
}

// RegisterCMSRoutes registers all currently enabled CMS modules on the Admin
// API group and the read-only public publishing surface on the same API
// process. Authentication and CSRF protect only the admin group; public reads
// intentionally remain unauthenticated.
func RegisterCMSRoutes(
	router *gin.RouterGroup,
	registration cmsRouteRegistration,
	identity identityRuntime,
) {
	if router == nil {
		return
	}
	protected := router.Group("")
	protected.Use(identity.authenticate, identity.csrf)
	registerPublishingRoutes(protected, registration.publishing, registration.guards)
	registerMediaRoutes(protected, registration, registration.guards)
	registerNavigationRoutes(protected, registration, registration.guards)
	registerDiscoverabilityRoutes(protected, registration, registration.guards)
	if registration.public != nil {
		publishinghttp.RegisterPublicRoutes(router, registration.public)
	}
	if registration.preview != nil {
		publishinghttp.RegisterPreviewExchangeRoute(router, registration.preview)
	}
}

func registerPublishingRoutes(
	router *gin.RouterGroup,
	handlers publishinghttp.RouteHandlers,
	guards cmsRouteGuards,
) {
	if handlers.Content != nil {
		router.GET("/publishing/pages", guards.pagesRead, handlers.Content.ListPages)
		router.POST("/publishing/pages", guards.pagesWrite, handlers.Content.CreatePage)
		router.GET("/publishing/pages/:page_id", guards.pagesRead, handlers.Content.GetPage)
		router.PATCH("/publishing/pages/:page_id", guards.pagesWrite, handlers.Content.Update)
		router.DELETE("/publishing/pages/:page_id", guards.pagesWrite, handlers.Content.Delete)
		router.GET("/publishing/posts", guards.pagesRead, handlers.Content.ListPosts)
		router.POST("/publishing/posts", guards.pagesWrite, handlers.Content.CreatePost)
		router.GET("/publishing/posts/:post_id", guards.pagesRead, handlers.Content.GetPost)
		router.PATCH("/publishing/posts/:post_id", guards.pagesWrite, handlers.Content.Update)
		router.DELETE("/publishing/posts/:post_id", guards.pagesWrite, handlers.Content.Delete)
	}
	if handlers.Revisions != nil {
		router.GET("/publishing/content/:content_id/revisions", guards.revisionsRead, handlers.Revisions.List)
		router.POST("/publishing/content/:content_id/revisions/:revision_id/rollback", guards.revisionsRollback, handlers.Revisions.Rollback)
	}
	if handlers.Workflow != nil {
		router.POST("/publishing/content/:content_id/workflow", guards.pagesReview, handlers.Workflow.Transition)
	}
	if handlers.Schedules != nil {
		router.GET("/publishing/schedules", guards.pagesRead, handlers.Schedules.List)
		router.POST("/publishing/schedules", guards.pagesPublish, handlers.Schedules.Create)
		router.DELETE("/publishing/schedules/:schedule_id", guards.pagesPublish, handlers.Schedules.Cancel)
	}
	if handlers.Preview != nil {
		router.POST("/publishing/content/:content_id/preview", guards.pagesRead, handlers.Preview.Issue)
	}
}

func registerMediaRoutes(
	router *gin.RouterGroup,
	registration cmsRouteRegistration,
	guards cmsRouteGuards,
) {
	if registration.uploads != nil {
		router.POST("/media/uploads", guards.mediaUploadsWrite, registration.uploads.Start)
		router.POST("/media/uploads/:upload_id/finalize", guards.mediaUploadsWrite, registration.uploads.Finalize)
	}
	if registration.assets != nil {
		router.GET("/media/assets", guards.mediaAssetsRead, registration.assets.List)
		router.DELETE("/media/assets/:asset_id", guards.mediaAssetsWrite, registration.assets.Delete)
	}
}

func registerNavigationRoutes(
	router *gin.RouterGroup,
	registration cmsRouteRegistration,
	guards cmsRouteGuards,
) {
	if registration.menus == nil {
		return
	}
	router.GET("/navigation/menus", guards.menusRead, registration.menus.List)
	router.POST("/navigation/menus", guards.menusWrite, registration.menus.Create)
	router.GET("/navigation/menus/:menu_id", guards.menusRead, registration.menus.Get)
	router.PATCH("/navigation/menus/:menu_id", guards.menusWrite, registration.menus.Update)
	router.DELETE("/navigation/menus/:menu_id", guards.menusWrite, registration.menus.Delete)
	router.POST("/navigation/menus/:menu_id/reorder", guards.menusWrite, registration.menus.Reorder)
}

func registerDiscoverabilityRoutes(
	router *gin.RouterGroup,
	registration cmsRouteRegistration,
	guards cmsRouteGuards,
) {
	if registration.redirects != nil {
		router.GET("/discoverability/redirects", guards.redirectsRead, registration.redirects.List)
		router.POST("/discoverability/redirects", guards.redirectsWrite, registration.redirects.Create)
		router.GET("/discoverability/redirects/:redirect_id", guards.redirectsRead, registration.redirects.Get)
		router.PATCH("/discoverability/redirects/:redirect_id", guards.redirectsWrite, registration.redirects.Update)
		router.DELETE("/discoverability/redirects/:redirect_id", guards.redirectsWrite, registration.redirects.Delete)
	}
	if registration.settings != nil {
		router.GET("/discoverability/seo", guards.seoRead, registration.settings.Get)
		router.PATCH("/discoverability/seo", guards.seoWrite, registration.settings.Update)
	}
	if registration.audits != nil {
		router.GET("/discoverability/audits", guards.auditsRead, registration.audits.List)
		router.POST("/discoverability/audits", guards.auditsRun, registration.audits.Run)
		router.GET("/discoverability/audits/:audit_id", guards.auditsRead, registration.audits.Get)
	}
}
