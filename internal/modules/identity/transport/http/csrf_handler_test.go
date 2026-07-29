package http

import (
	"context"
	"encoding/json"
	"errors"
	nethttp "net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	adminapi "github.com/korwlik23/go-api-starter/internal/generated/admin"
	sessionapp "github.com/korwlik23/go-api-starter/internal/modules/identity/application/session"
	"github.com/korwlik23/go-api-starter/internal/platform/middleware"
	"github.com/stretchr/testify/require"
)

func TestCSRFHandlerCreatesAnonymousSession(t *testing.T) {
	gin.SetMode(gin.TestMode)
	creator := &anonymousSessionCreatorStub{
		result: sessionapp.CreateAnonymousResult{
			SessionToken: "raw-session-token",
			CSRFToken:    "raw-csrf-token",
		},
	}
	handler, err := NewCSRFHandler(creator, func(ctx *gin.Context, token string) {
		ctx.Header("Set-Cookie", "session="+token+"; Path=/; HttpOnly")
	})
	require.NoError(t, err)

	router := gin.New()
	router.Use(middleware.RequestID(func() string { return "request-id" }))
	RegisterCSRFRoutes(router.Group("/api/v1"), handler)

	request := httptest.NewRequest(nethttp.MethodGet, "/api/v1/auth/csrf", nil)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	require.Equal(t, nethttp.StatusOK, response.Code)
	require.Equal(t, "no-store", response.Header().Get("Cache-Control"))
	require.Equal(
		t,
		"session=raw-session-token; Path=/; HttpOnly",
		response.Header().Get("Set-Cookie"),
	)
	var body adminapi.CSRFTokenResponse
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &body))
	require.Equal(t, "raw-csrf-token", body.CsrfToken)
	require.Equal(t, 1, creator.calls)
}

func TestCSRFHandlerReturnsSafeErrorWithoutSettingCookie(t *testing.T) {
	gin.SetMode(gin.TestMode)
	creator := &anonymousSessionCreatorStub{err: errors.New("private database failure")}
	handler, err := NewCSRFHandler(creator, func(ctx *gin.Context, token string) {
		ctx.Header("Set-Cookie", token)
	})
	require.NoError(t, err)

	router := gin.New()
	router.Use(middleware.RequestID(func() string { return "request-id" }))
	RegisterCSRFRoutes(router.Group("/api/v1"), handler)

	request := httptest.NewRequest(nethttp.MethodGet, "/api/v1/auth/csrf", nil)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	require.Equal(t, nethttp.StatusInternalServerError, response.Code)
	require.Empty(t, response.Header().Get("Set-Cookie"))
	require.NotContains(t, response.Body.String(), "private database failure")
	require.Contains(t, response.Body.String(), `"request_id":"request-id"`)
}

type anonymousSessionCreatorStub struct {
	result sessionapp.CreateAnonymousResult
	err    error
	calls  int
}

func (creator *anonymousSessionCreatorStub) Execute(
	context.Context,
) (sessionapp.CreateAnonymousResult, error) {
	creator.calls++
	return creator.result, creator.err
}
