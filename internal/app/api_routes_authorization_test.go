package app

import (
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
	})

	require.ElementsMatch(t, []string{
		"GET /api/v1/account",
		"GET /api/v1/authorization/permissions",
		"GET /api/v1/system/modules",
	}, routeKeys(router))
}
