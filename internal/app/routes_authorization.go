package app

import "github.com/gin-gonic/gin"

func RegisterAuthorizationRoutes(
	router *gin.RouterGroup,
	runtime authorizationRuntime,
	identity identityRuntime,
) {
	protected := router.Group("")
	protected.Use(identity.authenticate, identity.csrf)
	protected.GET(
		"/account",
		runtime.currentAccountGuard,
		runtime.currentAccount.Get,
	)
	protected.GET(
		"/authorization/permissions",
		runtime.effectiveGuard,
		runtime.effectivePermissions.GetEffective,
	)
	protected.GET("/system/modules", runtime.modulesGuard, runtime.modules.List)
	protected.POST(
		"/authorization/roles/:role_id/permissions",
		runtime.grantGuard,
		runtime.grantRolePermission.Grant,
	)
	protected.POST(
		"/authorization/roles/:role_id/assignments",
		runtime.assignmentGuard,
		runtime.roleAssignment.Assign,
	)
	protected.DELETE(
		"/authorization/assignments/:assignment_id",
		runtime.revokeGuard,
		runtime.roleAssignment.Revoke,
	)
}
