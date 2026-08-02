package app

import (
	"github.com/gin-gonic/gin"
	navigationhttp "github.com/korwlik23/go-api-starter/internal/modules/navigation/transport/http"
)

// RegisterNavigationRoutes composes the account-scoped menu administration
// surface behind the same session and CSRF boundary as other Admin routes.
// A disabled module passes nil and exposes no runtime routes.
func RegisterNavigationRoutes(router *gin.RouterGroup, handler *navigationhttp.MenuHandler, identity identityRuntime) {
	if router == nil || handler == nil {
		return
	}
	protected := router.Group("")
	protected.Use(identity.authenticate, identity.csrf)
	protected.GET("/navigation/menus", handler.List)
	protected.POST("/navigation/menus", handler.Create)
	protected.GET("/navigation/menus/:menu_id", handler.Get)
	protected.PATCH("/navigation/menus/:menu_id", handler.Update)
	protected.DELETE("/navigation/menus/:menu_id", handler.Delete)
	protected.POST("/navigation/menus/:menu_id/reorder", handler.Reorder)
}
