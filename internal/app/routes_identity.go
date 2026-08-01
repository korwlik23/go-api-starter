package app

import (
	"github.com/gin-gonic/gin"
	identityhttp "github.com/korwlik23/go-api-starter/internal/modules/identity/transport/http"
)

func RegisterIdentityRoutes(router *gin.RouterGroup, runtime identityRuntime) {
	protected := router.Group("")
	protected.Use(runtime.authenticate, runtime.csrf)
	identityhttp.RegisterRoutes(
		protected,
		runtime.handlers,
		runtime.rateLimits,
	)
}
