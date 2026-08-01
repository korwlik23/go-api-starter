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
	protected.GET(
		"/authorization/roles",
		runtime.rolesReadGuard,
		runtime.roleHandler.ListRoles,
	)
	protected.POST(
		"/authorization/roles",
		runtime.rolesManageGuard,
		runtime.roleHandler.CreateRole,
	)
	protected.PATCH(
		"/authorization/roles/:role_id",
		runtime.rolesManageGuard,
		runtime.roleHandler.UpdateRole,
	)
	protected.GET(
		"/authorization/roles/:role_id/permissions",
		runtime.rolePermissionsReadGuard,
		runtime.roleHandler.ListRolePermissions,
	)
	protected.GET(
		"/authorization/roles/:role_id/assignments",
		runtime.roleAssignmentsReadGuard,
		runtime.roleHandler.ListRoleAssignments,
	)
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

// RegisterAdministrationRoutes keeps the user and membership administration
// endpoints behind the same session/CSRF boundary as the authorization surface.
func RegisterAdministrationRoutes(
	router *gin.RouterGroup,
	runtime authorizationRuntime,
	identity identityRuntime,
) {
	if runtime.adminUsers == nil || runtime.memberships == nil {
		return
	}
	protected := router.Group("")
	protected.Use(identity.authenticate, identity.csrf)
	protected.GET("/users", runtime.adminUserReadGuard, runtime.adminUsers.List)
	protected.GET("/users/:user_id", runtime.adminUserReadGuard, runtime.adminUsers.Get)
	protected.PATCH("/users/:user_id/status", runtime.adminUserManageGuard, runtime.adminUsers.UpdateStatus)
	protected.GET("/accounts/:account_id/memberships", runtime.membershipReadGuard, runtime.memberships.List)
	protected.PATCH("/accounts/:account_id/memberships/:membership_id/status", runtime.membershipManageGuard, runtime.memberships.UpdateStatus)
	protected.GET("/audit/events", runtime.auditEventsReadGuard, runtime.auditEvents.List)
}
