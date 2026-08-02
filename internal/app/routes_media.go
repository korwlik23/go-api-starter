package app

import (
	"github.com/gin-gonic/gin"
	mediahttp "github.com/korwlik23/go-api-starter/internal/modules/media/transport/http"
)

// RegisterMediaRoutes composes media endpoints behind the same session and
// CSRF boundary as the other authenticated Admin API routes. A disabled media
// module passes nil handlers and therefore exposes no runtime routes.
func RegisterMediaRoutes(
	router *gin.RouterGroup,
	uploads *mediahttp.UploadHandler,
	assets *mediahttp.AssetHandler,
	identity identityRuntime,
) {
	if router == nil {
		return
	}
	if uploads == nil && assets == nil {
		return
	}
	protected := router.Group("")
	protected.Use(identity.authenticate, identity.csrf)
	if uploads != nil {
		protected.POST("/media/uploads", uploads.Start)
		protected.POST("/media/uploads/:upload_id/finalize", uploads.Finalize)
	}
	if assets != nil {
		protected.GET("/media/assets", assets.List)
		protected.DELETE("/media/assets/:asset_id", assets.Delete)
	}
}
