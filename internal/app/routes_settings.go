package app

import (
	"github.com/gin-gonic/gin"
)

func RegisterSettingsRoutes(router *gin.RouterGroup, runtime settingsRuntime, identity identityRuntime) {
	if router == nil || runtime.handler == nil || runtime.readGuard == nil || runtime.manageGuard == nil {
		return
	}
	protected := router.Group("")
	protected.Use(identity.authenticate, identity.csrf)
	protected.GET("/settings", runtime.readGuard, runtime.handler.List)
	protected.PUT("/settings/:key", runtime.manageGuard, runtime.handler.Update)
}
