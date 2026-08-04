package http

import "github.com/gin-gonic/gin"

func RegisterRoutes(router *gin.RouterGroup, handler *Handler) {
	if router == nil || handler == nil {
		return
	}
	router.GET("/notifications", handler.List)
	router.PATCH("/notifications/:notification_id/read", handler.MarkRead)
	router.POST("/notifications/read-all", handler.MarkAllRead)
}
