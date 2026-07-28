package httpserver

import "github.com/gin-gonic/gin"

func NewRouter(readiness ReadyChecker) *gin.Engine {
	router := gin.New()
	if err := router.SetTrustedProxies(nil); err != nil {
		panic("failed to disable implicit trusted proxies")
	}
	RegisterLiveness(router)
	RegisterReadiness(router, readiness)
	return router
}
