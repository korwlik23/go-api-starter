package http

import "github.com/gin-gonic/gin"

type RouteHandlers struct {
	Content   *ContentHandler
	Revisions *RevisionHandler
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
}
