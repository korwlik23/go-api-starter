package app

import (
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"
	identityhttp "github.com/korwlik23/go-api-starter/internal/modules/identity/transport/http"
	"github.com/stretchr/testify/require"
)

func TestIdentityProfileRoutesIncludePasswordChangeWhenConfigured(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	noOp := func(ctx *gin.Context) { ctx.Next() }

	RegisterIdentityProfileRoutes(router.Group("/api/v1"), identityRuntime{
		profile:  &identityhttp.ProfileHandler{},
		password: &identityhttp.PasswordHandler{},
		authenticate: noOp,
		csrf:         noOp,
	}, authorizationRuntime{
		profileReadGuard:    noOp,
		profileManageGuard:  noOp,
		passwordManageGuard: noOp,
	})

	routes := make(map[string]struct{}, len(router.Routes()))
	for _, route := range router.Routes() {
		routes[route.Method+" "+route.Path] = struct{}{}
	}
	require.Contains(t, routes, http.MethodGet+" /api/v1/me/profile")
	require.Contains(t, routes, http.MethodPatch+" /api/v1/me/profile")
	require.Contains(t, routes, http.MethodPatch+" /api/v1/me/password")
}
