package app

import (
	"github.com/gin-gonic/gin"
	settingshttp "github.com/korwlik23/go-api-starter/internal/modules/settings/transport/http"
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

var _ = settingshttp.RegisterRoutes
