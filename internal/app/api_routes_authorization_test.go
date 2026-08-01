package app

import (
	nethttp "net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	accountshttp "github.com/korwlik23/go-api-starter/internal/modules/accounts/transport/http"
	authorizationhttp "github.com/korwlik23/go-api-starter/internal/modules/authorization/transport/http"
	operationshttp "github.com/korwlik23/go-api-starter/internal/modules/operations/transport/http"
	"github.com/stretchr/testify/require"
)

func TestAuthorizationCompositionRegistersProtectedReadRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	noOp := func(ctx *gin.Context) { ctx.Next() }

	RegisterAuthorizationRoutes(router.Group("/api/v1"), authorizationRuntime{
		currentAccount:       &accountshttp.CurrentAccountHandler{},
		effectivePermissions: &authorizationhttp.PermissionHandler{},
		modules:              &operationshttp.ModuleHandler{},
		currentAccountGuard:  noOp,
		effectiveGuard:       noOp,
		modulesGuard:         noOp,
	}, identityRuntime{
		authenticate: noOp,
		csrf:         noOp,
	})

	require.ElementsMatch(t, []string{
		"GET /api/v1/account",
		"GET /api/v1/authorization/permissions",
		"GET /api/v1/system/modules",
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
