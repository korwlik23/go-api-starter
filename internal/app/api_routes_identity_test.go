package app

import (
	"testing"

	"github.com/gin-gonic/gin"
	identityhttp "github.com/korwlik23/go-api-starter/internal/modules/identity/transport/http"
	"github.com/stretchr/testify/require"
)

func TestIdentityCompositionRegistersTheCompleteContract(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	noOp := func(ctx *gin.Context) { ctx.Next() }
	runtime := identityRuntime{
		handlers: identityhttp.RouteHandlers{
			CSRF:            &identityhttp.CSRFHandler{},
			Login:           &identityhttp.LoginHandler{},
			MFA:             &identityhttp.MFAHandler{},
			MFAConfirmation: &identityhttp.MFAConfirmationHandler{},
			MFAEnrollment:   &identityhttp.MFAEnrollmentHandler{},
			Recovery:        &identityhttp.RecoveryHandler{},
			Session:         &identityhttp.SessionHandler{},
			SessionQuery:    &identityhttp.SessionQueryHandler{},
		},
		rateLimits: identityhttp.RateLimits{
			Baseline: noOp, Login: noOp, Recovery: noOp, MFA: noOp,
		},
		authenticate: noOp,
		csrf:         noOp,
	}

	RegisterIdentityRoutes(router.Group("/api/v1"), runtime)

	require.Len(t, router.Routes(), 12)
	require.Contains(t, routeKeys(router), "GET /api/v1/auth/csrf")
	require.Contains(t, routeKeys(router), "POST /api/v1/auth/login")
	require.Contains(t, routeKeys(router), "POST /api/v1/auth/mfa/verify")
	require.Contains(t, routeKeys(router), "DELETE /api/v1/auth/sessions/:session_id")
}

func routeKeys(router *gin.Engine) []string {
	routes := router.Routes()
	keys := make([]string, 0, len(routes))
	for _, route := range routes {
		keys = append(keys, route.Method+" "+route.Path)
	}
	return keys
}
