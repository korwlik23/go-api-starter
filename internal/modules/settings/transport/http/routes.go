package http

import "github.com/gin-gonic/gin"

func RegisterRoutes(router *gin.RouterGroup, handler *Handler) {
	if router == nil || handler == nil {
		return
	}
	router.GET("/settings", handler.List)
	router.PUT("/settings/:key", handler.Update)
}
