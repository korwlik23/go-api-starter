package app

import (
	"context"
	nethttp "net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	accountshttp "github.com/korwlik23/go-api-starter/internal/modules/accounts/transport/http"
	authorizationapp "github.com/korwlik23/go-api-starter/internal/modules/authorization/application"
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

	RegisterAuthorizationRoutes(router.Group("/api/v1"), authorizationRuntime{
		currentAccount:       &accountshttp.CurrentAccountHandler{},
		effectivePermissions: &authorizationhttp.PermissionHandler{},
		modules:              &operationshttp.ModuleHandler{},
		currentAccountGuard:  noOp,
		effectiveGuard:       noOp,
		modulesGuard:         noOp,
		grantGuard:           noOp,
		grantRolePermission:  grantHandler,
		assignmentGuard:      noOp,
		revokeGuard:          noOp,
		roleAssignment:       assignmentHandler,
	}, identityRuntime{
		authenticate: noOp,
		csrf:         noOp,
	})

	require.ElementsMatch(t, []string{
		"GET /api/v1/account",
		"GET /api/v1/authorization/permissions",
		"GET /api/v1/system/modules",
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
		currentAccount:       &accountshttp.CurrentAccountHandler{},
		effectivePermissions: &authorizationhttp.PermissionHandler{},
		modules:              &operationshttp.ModuleHandler{},
		currentAccountGuard:  guard,
		effectiveGuard:       guard,
		modulesGuard:         guard,
		grantGuard:           guard,
		grantRolePermission:  mustGrantRolePermissionHandler(t),
		assignmentGuard:      guard,
		revokeGuard:          guard,
		roleAssignment:       mustRoleAssignmentHandler(t),
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
