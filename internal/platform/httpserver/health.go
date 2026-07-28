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
