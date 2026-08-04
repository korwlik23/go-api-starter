package app

import (
	"github.com/gin-gonic/gin"
)

func RegisterNotificationsRoutes(router *gin.RouterGroup, runtime notificationsRuntime, identity identityRuntime) {
	if router == nil || runtime.handler == nil || runtime.readGuard == nil || runtime.manageGuard == nil {
		return
	}
	protected := router.Group("")
	protected.Use(identity.authenticate, identity.csrf)
	protected.GET("/notifications", runtime.readGuard, runtime.handler.List)
	protected.PATCH("/notifications/:notification_id/read", runtime.manageGuard, runtime.handler.MarkRead)
	protected.POST("/notifications/read-all", runtime.manageGuard, runtime.handler.MarkAllRead)
}
