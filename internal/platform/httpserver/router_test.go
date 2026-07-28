package httpserver

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestA11ARouter(t *testing.T) {
	t.Parallel()

	router := NewRouter(nil)
	router.GET("/client-ip", func(ctx *gin.Context) {
		ctx.String(http.StatusOK, ctx.ClientIP())
	})

	request := httptest.NewRequest(http.MethodGet, "/client-ip", nil)
	request.RemoteAddr = "203.0.113.10:3210"
	request.Header.Set("X-Forwarded-For", "198.51.100.20")
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	require.Equal(t, http.StatusOK, response.Code)
	require.Equal(t, "203.0.113.10", response.Body.String())
}
