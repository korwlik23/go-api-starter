package httpserver

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

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

func TestA9(t *testing.T) {
	t.Parallel()

	t.Run("critical failure is unavailable", func(t *testing.T) {
		t.Parallel()

		readiness := Readiness{
			Timeout: 50 * time.Millisecond,
			Probes: []Probe{
				{
					Name:     "database",
					Critical: true,
					Check: func(context.Context) error {
						return errors.New("private database detail")
					},
				},
			},
		}

		response := serveReadiness(readiness)
		require.Equal(t, http.StatusServiceUnavailable, response.Code)
		require.JSONEq(t, `{"status":"unavailable"}`, response.Body.String())
		require.NotContains(t, response.Body.String(), "database")
		require.NotContains(t, response.Body.String(), "private")
	})

	t.Run("critical timeout is unavailable", func(t *testing.T) {
		t.Parallel()

		readiness := Readiness{
			Timeout: 20 * time.Millisecond,
			Probes: []Probe{
				{
					Name:     "database",
					Critical: true,
					Check: func(ctx context.Context) error {
						<-ctx.Done()
						return ctx.Err()
					},
				},
			},
		}

		startedAt := time.Now()
		response := serveReadiness(readiness)
		require.Equal(t, http.StatusServiceUnavailable, response.Code)
		require.Less(t, time.Since(startedAt), 500*time.Millisecond)
	})

	t.Run("optional failure stays ready", func(t *testing.T) {
		t.Parallel()

		readiness := Readiness{
			Timeout: 50 * time.Millisecond,
			Probes: []Probe{
				{
					Name:     "database",
					Critical: true,
					Check: func(context.Context) error {
						return nil
					},
				},
				{
					Name:     "redis",
					Critical: false,
					Check: func(context.Context) error {
						return errors.New("private redis detail")
					},
				},
			},
		}

		response := serveReadiness(readiness)
		require.Equal(t, http.StatusOK, response.Code)
		require.JSONEq(t, `{"status":"ready"}`, response.Body.String())
	})
}

func serveReadiness(readiness Readiness) *httptest.ResponseRecorder {
	router := gin.New()
	RegisterReadiness(router, readiness)

	request := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	return response
}
