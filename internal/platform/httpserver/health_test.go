package httpserver

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestA8(t *testing.T) {
	t.Parallel()

	router := gin.New()
	RegisterLiveness(router)

	request := httptest.NewRequest(http.MethodGet, "/livez", nil)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	require.Equal(t, http.StatusOK, response.Code)
	require.Equal(t, "application/json; charset=utf-8", response.Header().Get("Content-Type"))

	var body map[string]string
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &body))
	require.Equal(t, map[string]string{"status": "ok"}, body)
}
