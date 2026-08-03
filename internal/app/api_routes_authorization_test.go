package app

import (
	"context"
	nethttp "net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	accountshttp "github.com/korwlik23/go-api-starter/internal/modules/accounts/transport/http"
	authorizationapp "github.com/korwlik23/go-api-starter/internal/modules/authorization/application"
	authorizationdomain "github.com/korwlik23/go-api-starter/internal/modules/authorization/domain"
	"github.com/korwlik23/go-api-starter/internal/modules/authorization/ports"
	authorizationhttp "github.com/korwlik23/go-api-starter/internal/modules/authorization/transport/http"
	operationshttp "github.com/korwlik23/go-api-starter/internal/modules/operations/transport/http"
	"github.com/stretchr/testify/require"
)

func TestAuthorizationCompositionRegistersProtectedReadRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	noOp := func(ctx *gin.Context) { ctx.Next() }
	grantHandler, err := authorizationhttp.NewGrantRolePermissionHandler(routeGrantServiceStub{})
	require.NoError(t, err)
	assignmentHandler, err := authorizationhttp.NewRoleAssignmentHandler(routeRoleAssignmentServiceStub{})
	require.NoError(t, err)
	roleHandler := mustRoleHandler(t)

	RegisterAuthorizationRoutes(router.Group("/api/v1"), authorizationRuntime{
		currentAccount:           &accountshttp.CurrentAccountHandler{},
		effectivePermissions:     &authorizationhttp.PermissionHandler{},
		modules:                  &operationshttp.ModuleHandler{},
		currentAccountGuard:      noOp,
		effectiveGuard:           noOp,
		modulesGuard:             noOp,
		grantGuard:               noOp,
		grantRolePermission:      grantHandler,
		assignmentGuard:          noOp,
		revokeGuard:              noOp,
		roleAssignment:           assignmentHandler,
		rolesReadGuard:           noOp,
		rolesManageGuard:         noOp,
		rolePermissionsReadGuard: noOp,
		roleAssignmentsReadGuard: noOp,
		roleHandler:              roleHandler,
	}, identityRuntime{
		authenticate: noOp,
		csrf:         noOp,
	})

	require.ElementsMatch(t, []string{
		"GET /api/v1/account",
		"GET /api/v1/authorization/effective-permissions",
		"GET /api/v1/operations/modules",
		"GET /api/v1/authorization/roles",
		"POST /api/v1/authorization/roles",
		"PATCH /api/v1/authorization/roles/:role_id",
		"GET /api/v1/authorization/roles/:role_id/permissions",
		"GET /api/v1/authorization/roles/:role_id/assignments",
		"POST /api/v1/authorization/roles/:role_id/permissions",
		"POST /api/v1/authorization/roles/:role_id/assignments",
		"DELETE /api/v1/authorization/assignments/:assignment_id",
	}, routeKeys(router))
}

func TestAuthorizationCompositionAppliesIdentitySecurityBeforePermissionGuards(
	t *testing.T,
) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	guardReached := false
	guard := func(ctx *gin.Context) {
		guardReached = true
		ctx.Next()
	}
	authenticate := func(ctx *gin.Context) {
		ctx.AbortWithStatus(nethttp.StatusUnauthorized)
	}

	RegisterAuthorizationRoutes(router.Group("/api/v1"), authorizationRuntime{
		currentAccount:           &accountshttp.CurrentAccountHandler{},
		effectivePermissions:     &authorizationhttp.PermissionHandler{},
		modules:                  &operationshttp.ModuleHandler{},
		currentAccountGuard:      guard,
		effectiveGuard:           guard,
		modulesGuard:             guard,
		grantGuard:               guard,
		grantRolePermission:      mustGrantRolePermissionHandler(t),
		assignmentGuard:          guard,
		revokeGuard:              guard,
		roleAssignment:           mustRoleAssignmentHandler(t),
		rolesReadGuard:           guard,
		rolesManageGuard:         guard,
		rolePermissionsReadGuard: guard,
		roleAssignmentsReadGuard: guard,
		roleHandler:              mustRoleHandler(t),
	}, identityRuntime{
		authenticate: authenticate,
		csrf:         func(ctx *gin.Context) { ctx.Next() },
	})

	request := httptest.NewRequest(nethttp.MethodGet, "/api/v1/account", nil)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	require.Equal(t, nethttp.StatusUnauthorized, response.Code)
	require.False(t, guardReached)
}

type routeGrantServiceStub struct{}

func (routeGrantServiceStub) Execute(context.Context, authorizationapp.GrantRolePermissionCommand) (authorizationapp.GrantRolePermissionResult, error) {
	return authorizationapp.GrantRolePermissionResult{}, nil
}

type routeRoleAssignmentServiceStub struct{}

func (routeRoleAssignmentServiceStub) Assign(context.Context, authorizationapp.AssignRoleCommand) (authorizationapp.RoleAssignmentResult, error) {
	return authorizationapp.RoleAssignmentResult{}, nil
}

func (routeRoleAssignmentServiceStub) Revoke(context.Context, authorizationapp.RevokeRoleCommand) (authorizationapp.RoleRevokeResult, error) {
	return authorizationapp.RoleRevokeResult{}, nil
}

func mustGrantRolePermissionHandler(t *testing.T) *authorizationhttp.GrantRolePermissionHandler {
	handler, err := authorizationhttp.NewGrantRolePermissionHandler(routeGrantServiceStub{})
	require.NoError(t, err)
	return handler
}

func mustRoleAssignmentHandler(t *testing.T) *authorizationhttp.RoleAssignmentHandler {
	handler, err := authorizationhttp.NewRoleAssignmentHandler(routeRoleAssignmentServiceStub{})
	require.NoError(t, err)
	return handler
}

type routeRoleQueryStub struct{}

func (routeRoleQueryStub) ListRoles(context.Context, ports.RoleListFilter) (ports.RolePage, error) {
	return ports.RolePage{}, nil
}

func (routeRoleQueryStub) ListRolePermissions(context.Context, string) (ports.RolePermissionList, error) {
	return ports.RolePermissionList{}, nil
}

func (routeRoleQueryStub) ListRoleAssignments(context.Context, string, ports.RolePageRequest) (ports.RoleAssignmentPage, error) {
	return ports.RoleAssignmentPage{}, nil
}

type routeRoleManagementStub struct{}

func (routeRoleManagementStub) Create(context.Context, authorizationapp.CreateRoleCommand) (authorizationdomain.Role, error) {
	return authorizationdomain.Role{}, nil
}

func (routeRoleManagementStub) Update(context.Context, authorizationapp.UpdateRoleCommand) (authorizationdomain.Role, error) {
	return authorizationdomain.Role{}, nil
}

func mustRoleHandler(t *testing.T) *authorizationhttp.RoleHandler {
	handler, err := authorizationhttp.NewRoleHandler(routeRoleQueryStub{}, routeRoleManagementStub{})
	require.NoError(t, err)
	return handler
}
