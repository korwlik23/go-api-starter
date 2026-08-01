package http

import "github.com/gin-gonic/gin"

type RouteHandlers struct {
	Content *ContentHandler
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
}
