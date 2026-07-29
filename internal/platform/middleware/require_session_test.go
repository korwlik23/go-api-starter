package middleware

import (
	"context"
	nethttp "net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	identitydomain "github.com/korwlik23/go-api-starter/internal/modules/identity/domain"
	"github.com/stretchr/testify/require"
)

func TestRequireSessionRejectsAnonymousRequest(t *testing.T) {
	t.Parallel()

	router := requireSessionTestRouter(nil, identitydomain.SessionStateAuthenticated)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(nethttp.MethodGet, "/", nil))

	require.Equal(t, nethttp.StatusUnauthorized, response.Code)
	require.Contains(t, response.Body.String(), `"code":"authentication_required"`)
}

func TestRequireSessionRejectsDisallowedSessionState(t *testing.T) {
	t.Parallel()

	session := &identitydomain.Session{State: identitydomain.SessionStateMFAPending}
	router := requireSessionTestRouter(
		session,
		identitydomain.SessionStateAuthenticated,
	)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(nethttp.MethodGet, "/", nil))

	require.Equal(t, nethttp.StatusForbidden, response.Code)
	require.Contains(t, response.Body.String(), `"code":"session_state_forbidden"`)
}

func TestRequireSessionAcceptsAllowedSessionState(t *testing.T) {
	t.Parallel()

	session := &identitydomain.Session{State: identitydomain.SessionStateAuthenticated}
	router := requireSessionTestRouter(
		session,
		identitydomain.SessionStateAuthenticated,
	)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(nethttp.MethodGet, "/", nil))

	require.Equal(t, nethttp.StatusNoContent, response.Code)
}

func requireSessionTestRouter(
	session *identitydomain.Session,
	allowedStates ...identitydomain.SessionState,
) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(RequestID(func() string { return "request-id" }))
	if session != nil {
		router.Use(func(ctx *gin.Context) {
			requestContext := context.WithValue(
				ctx.Request.Context(),
				sessionContextKey{},
				*session,
			)
			ctx.Request = ctx.Request.WithContext(requestContext)
			ctx.Next()
		})
	}
	router.Use(RequireSession(allowedStates...))
	router.GET("/", func(ctx *gin.Context) {
		ctx.Status(nethttp.StatusNoContent)
	})
	return router
}
