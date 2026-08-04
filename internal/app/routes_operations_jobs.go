package app

import (
	"github.com/gin-gonic/gin"
)

func RegisterOperationsJobRoutes(router *gin.RouterGroup, runtime operationsJobsRuntime, identity identityRuntime) {
	if router == nil || runtime.handler == nil || runtime.readGuard == nil || runtime.manageGuard == nil {
		return
	}
	protected := router.Group("")
	protected.Use(identity.authenticate, identity.csrf)
	protected.GET("/operations/jobs", runtime.readGuard, runtime.handler.List)
	protected.POST("/operations/jobs", runtime.manageGuard, runtime.handler.Enqueue)
	protected.POST("/operations/jobs/:job_id/retry", runtime.manageGuard, runtime.handler.Retry)
}
