package app

import "github.com/gin-gonic/gin"

func RegisterAuthorizationRoutes(
	router *gin.RouterGroup,
	runtime authorizationRuntime,
) {
	router.GET("/account", runtime.currentAccountGuard, runtime.currentAccount.Get)
	router.GET(
		"/authorization/permissions",
		runtime.effectiveGuard,
		runtime.effectivePermissions.GetEffective,
	)
	router.GET("/system/modules", runtime.modulesGuard, runtime.modules.List)
}
