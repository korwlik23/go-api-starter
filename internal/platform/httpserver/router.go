package httpserver

import "github.com/gin-gonic/gin"

func NewRouter(readiness ReadyChecker) *gin.Engine {
	router := gin.New()
	RegisterLiveness(router)
	RegisterReadiness(router, readiness)
	return router
}
