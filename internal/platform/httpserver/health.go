package httpserver

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func RegisterLiveness(routes gin.IRoutes) {
	routes.GET("/livez", func(ctx *gin.Context) {
		ctx.Header("Cache-Control", "no-store")
		ctx.JSON(http.StatusOK, gin.H{"status": "ok"})
	})
}

func RegisterReadiness(routes gin.IRoutes, readiness Readiness) {
	routes.GET("/readyz", func(ctx *gin.Context) {
		ctx.Header("Cache-Control", "no-store")
		if !readiness.Ready(ctx.Request.Context()) {
			ctx.JSON(http.StatusServiceUnavailable, gin.H{"status": "unavailable"})
			return
		}
		ctx.JSON(http.StatusOK, gin.H{"status": "ready"})
	})
}
