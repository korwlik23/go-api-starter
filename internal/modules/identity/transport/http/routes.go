package http

import "github.com/gin-gonic/gin"

func RegisterCSRFRoutes(router *gin.RouterGroup, handler *CSRFHandler) {
	router.GET("/auth/csrf", handler.GetToken)
}
