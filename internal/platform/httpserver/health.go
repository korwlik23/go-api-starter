package httpserver

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
)

type ReadyChecker interface {
	Ready(context.Context) bool
}

func RegisterLiveness(routes gin.IRoutes) {
	routes.GET("/livez", func(ctx *gin.Context) {
		ctx.Header("Cache-Control", "no-store")
		ctx.JSON(http.StatusOK, gin.H{"status": "ok"})
	})
}

func RegisterReadiness(routes gin.IRoutes, readiness ReadyChecker) {
	routes.GET("/readyz", func(ctx *gin.Context) {
		ctx.Header("Cache-Control", "no-store")
		if readiness == nil || !readiness.Ready(ctx.Request.Context()) {
			ctx.JSON(http.StatusServiceUnavailable, gin.H{"status": "unavailable"})
			return
		}
		ctx.JSON(http.StatusOK, gin.H{"status": "ready"})
	})
}
