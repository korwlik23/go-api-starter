package app

import (
	"testing"

	"github.com/gin-gonic/gin"
	discoverabilityhttp "github.com/korwlik23/go-api-starter/internal/modules/discoverability/transport/http"
	"github.com/stretchr/testify/require"
)

func TestM3B5DiscoverabilityRoutesApplyIdentityBoundaryAndExposeContract(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	noOp := func(ctx *gin.Context) { ctx.Next() }
	RegisterDiscoverabilityRoutes(router.Group("/api/v1"), &discoverabilityhttp.RedirectHandler{}, &discoverabilityhttp.SettingsHandler{}, identityRuntime{authenticate: noOp, csrf: noOp})
	require.ElementsMatch(t, []string{
		"GET /api/v1/discoverability/redirects", "POST /api/v1/discoverability/redirects", "GET /api/v1/discoverability/redirects/:redirect_id",
		"PATCH /api/v1/discoverability/redirects/:redirect_id", "DELETE /api/v1/discoverability/redirects/:redirect_id", "GET /api/v1/discoverability/seo", "PATCH /api/v1/discoverability/seo",
	}, routeKeys(router))
}

func TestM3B5DisabledDiscoverabilityRoutesAreNotRegistered(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	RegisterDiscoverabilityRoutes(router.Group("/api/v1"), nil, nil, identityRuntime{})
	require.Empty(t, router.Routes())
}

func TestM3B6DiscoverabilityAuditRoutesExposeProtectedContract(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	noOp := func(ctx *gin.Context) { ctx.Next() }
	RegisterDiscoverabilityAuditRoutes(router.Group("/api/v1"), &discoverabilityhttp.AuditHandler{}, identityRuntime{authenticate: noOp, csrf: noOp})
	require.ElementsMatch(t, []string{"GET /api/v1/discoverability/audits", "POST /api/v1/discoverability/audits", "GET /api/v1/discoverability/audits/:audit_id"}, routeKeys(router))
}
