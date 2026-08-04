package app

import "github.com/gin-gonic/gin"

func RegisterIdentityProfileRoutes(router *gin.RouterGroup, identity identityRuntime, authorization authorizationRuntime) {
	if router == nil || identity.profile == nil || authorization.profileReadGuard == nil || authorization.profileManageGuard == nil {
		return
	}
	protected := router.Group("")
	protected.Use(identity.authenticate, identity.csrf)
	protected.GET("/me/profile", authorization.profileReadGuard, identity.profile.Get)
	protected.PATCH("/me/profile", authorization.profileManageGuard, identity.profile.Update)
}
