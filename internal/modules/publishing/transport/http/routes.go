package http

import "github.com/gin-gonic/gin"

type RouteHandlers struct {
	Content   *ContentHandler
	Revisions *RevisionHandler
	Workflow  *WorkflowHandler
	Schedules *ScheduleHandler
	Preview   *PreviewHandler
}

// RegisterRoutes exposes the draft content mutation surface. Authentication,
// CSRF, account resolution and permission guards are composed by the parent
// application route group so a disabled module can omit this registration.
func RegisterRoutes(router *gin.RouterGroup, handlers RouteHandlers) {
	router.POST("/publishing/pages", handlers.Content.CreatePage)
	router.PATCH("/publishing/pages/:page_id", handlers.Content.Update)
	router.DELETE("/publishing/pages/:page_id", handlers.Content.Delete)
	router.POST("/publishing/posts", handlers.Content.CreatePost)
	router.PATCH("/publishing/posts/:post_id", handlers.Content.Update)
	router.DELETE("/publishing/posts/:post_id", handlers.Content.Delete)
	if handlers.Revisions != nil {
		router.GET("/publishing/content/:content_id/revisions", handlers.Revisions.List)
		router.POST("/publishing/content/:content_id/revisions/:revision_id/rollback", handlers.Revisions.Rollback)
	}
	if handlers.Workflow != nil {
		router.POST("/publishing/content/:content_id/workflow", handlers.Workflow.Transition)
	}
	if handlers.Schedules != nil {
		router.GET("/publishing/schedules", handlers.Schedules.List)
		router.POST("/publishing/schedules", handlers.Schedules.Create)
		router.DELETE("/publishing/schedules/:schedule_id", handlers.Schedules.Cancel)
	}
	if handlers.Preview != nil {
		router.POST("/publishing/content/:content_id/preview", handlers.Preview.Issue)
	}
}

// RegisterPublicRoutes exposes read-only published content. It intentionally
// has no authentication or CSRF middleware; account and locale resolution are
// supplied when the public runtime is composed.
func RegisterPublicRoutes(router *gin.RouterGroup, handler *PublicHandler) {
	if router == nil || handler == nil {
		return
	}
	router.GET("/public/content/:locale/*path", handler.GetContent)
	router.GET("/public/posts/:locale", handler.ListPosts)
	router.GET("/public/posts/:locale/:slug", handler.GetPost)
	router.GET("/public/menus/:locale/:location", handler.GetMenu)
}

func RegisterPreviewExchangeRoute(router *gin.RouterGroup, handler *PreviewHandler) {
	if router == nil || handler == nil {
		return
	}
	router.POST("/public/previews/exchange", handler.Exchange)
}
