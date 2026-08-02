package app

import (
	"github.com/gin-gonic/gin"
	discoverabilityhttp "github.com/korwlik23/go-api-starter/internal/modules/discoverability/transport/http"
)

// RegisterDiscoverabilityRoutes composes the account-scoped redirects and SEO
// settings surface behind the same session and CSRF boundary as other Admin
// API routes. Disabled handlers are represented by nil and expose no routes.
func RegisterDiscoverabilityRoutes(
	router *gin.RouterGroup,
	redirects *discoverabilityhttp.RedirectHandler,
	settings *discoverabilityhttp.SettingsHandler,
	identity identityRuntime,
) {
	if router == nil || (redirects == nil && settings == nil) {
		return
	}
	protected := router.Group("")
	protected.Use(identity.authenticate, identity.csrf)
	if redirects != nil {
		protected.GET("/discoverability/redirects", redirects.List)
		protected.POST("/discoverability/redirects", redirects.Create)
		protected.GET("/discoverability/redirects/:redirect_id", redirects.Get)
		protected.PATCH("/discoverability/redirects/:redirect_id", redirects.Update)
		protected.DELETE("/discoverability/redirects/:redirect_id", redirects.Delete)
	}
	if settings != nil {
		protected.GET("/discoverability/seo", settings.Get)
		protected.PATCH("/discoverability/seo", settings.Update)
	}
}
