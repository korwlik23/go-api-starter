package middleware

import (
	nethttp "net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestCORSAllowsExactCredentialedAdminOrigin(t *testing.T) {
	router := corsTestRouter(t, []string{"https://admin.example.com"}, nil)
	request := httptest.NewRequest(nethttp.MethodGet, "/resource", nil)
	request.Header.Set("Origin", "https://admin.example.com")
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	require.Equal(t, nethttp.StatusNoContent, response.Code)
	require.Equal(
		t,
		"https://admin.example.com",
		response.Header().Get("Access-Control-Allow-Origin"),
	)
	require.Equal(t, "true", response.Header().Get("Access-Control-Allow-Credentials"))
	require.Contains(t, response.Header().Values("Vary"), "Origin")
}

func TestCORSDoesNotReflectForeignOrigins(t *testing.T) {
	router := corsTestRouter(t, []string{"https://admin.example.com"}, nil)
	request := httptest.NewRequest(nethttp.MethodGet, "/resource", nil)
	request.Header.Set("Origin", "https://admin.example.com.evil.test")
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	require.Equal(t, nethttp.StatusNoContent, response.Code)
	require.Empty(t, response.Header().Get("Access-Control-Allow-Origin"))
	require.Empty(t, response.Header().Get("Access-Control-Allow-Credentials"))
	require.Contains(t, response.Header().Values("Vary"), "Origin")
}

func TestCORSPreservesExistingVaryHeaders(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cors, err := NewCORS([]string{"https://admin.example.com"})
	require.NoError(t, err)
	router := gin.New()
	router.Use(func(ctx *gin.Context) {
		ctx.Header("Vary", "Accept-Encoding")
		ctx.Next()
	})
	router.Use(cors)
	router.GET("/resource", func(ctx *gin.Context) {
		ctx.Status(nethttp.StatusNoContent)
	})

	request := httptest.NewRequest(nethttp.MethodGet, "/resource", nil)
	request.Header.Set("Origin", "https://admin.example.com")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	require.ElementsMatch(
		t,
		[]string{"Accept-Encoding", "Origin"},
		response.Header().Values("Vary"),
	)
}

func TestCORSHandlesOnlyAllowedPreflight(t *testing.T) {
	handlerCalls := 0
	router := corsTestRouter(
		t,
		[]string{"https://admin.example.com"},
		func() { handlerCalls++ },
	)

	allowed := httptest.NewRequest(nethttp.MethodOptions, "/resource", nil)
	allowed.Header.Set("Origin", "https://admin.example.com")
	allowed.Header.Set("Access-Control-Request-Method", nethttp.MethodPost)
	allowed.Header.Set("Access-Control-Request-Headers", "Content-Type, X-CSRF-Token")
	allowedResponse := httptest.NewRecorder()
	router.ServeHTTP(allowedResponse, allowed)

	require.Equal(t, nethttp.StatusNoContent, allowedResponse.Code)
	require.Equal(t, 0, handlerCalls)
	require.Equal(
		t,
		"GET, HEAD, POST, PUT, PATCH, DELETE, OPTIONS",
		allowedResponse.Header().Get("Access-Control-Allow-Methods"),
	)
	require.Contains(
		t,
		allowedResponse.Header().Get("Access-Control-Allow-Headers"),
		"X-CSRF-Token",
	)

	foreign := httptest.NewRequest(nethttp.MethodOptions, "/resource", nil)
	foreign.Header.Set("Origin", "https://evil.example.com")
	foreign.Header.Set("Access-Control-Request-Method", nethttp.MethodPost)
	foreignResponse := httptest.NewRecorder()
	router.ServeHTTP(foreignResponse, foreign)

	require.Equal(t, nethttp.StatusForbidden, foreignResponse.Code)
	require.Equal(t, 0, handlerCalls)
	require.Empty(t, foreignResponse.Header().Get("Access-Control-Allow-Origin"))
}

func TestCORSRejectsWildcardAndMalformedConfiguration(t *testing.T) {
	for _, origins := range [][]string{
		{"*"},
		{"https://admin.example.com/path"},
		{"https://admin.example.com", "https://admin.example.com"},
		{},
	} {
		_, err := NewCORS(origins)
		require.ErrorIs(t, err, ErrInvalidCORSConfig)
	}
}

func corsTestRouter(
	t *testing.T,
	origins []string,
	onHandler func(),
) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	cors, err := NewCORS(origins)
	require.NoError(t, err)

	router := gin.New()
	router.Use(cors)
	router.Any("/resource", func(ctx *gin.Context) {
		if onHandler != nil {
			onHandler()
		}
		ctx.Status(nethttp.StatusNoContent)
	})
	return router
}
