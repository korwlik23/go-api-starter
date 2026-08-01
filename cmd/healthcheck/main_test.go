package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestC0(t *testing.T) {
	t.Run("ready 2xx succeeds", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(
			writer http.ResponseWriter,
			request *http.Request,
		) {
			require.Equal(t, "/readyz", request.URL.Path)
			writer.WriteHeader(http.StatusNoContent)
		}))
		defer server.Close()

		require.NoError(t, check(
			context.Background(),
			server.URL+"/readyz",
			500*time.Millisecond,
		))
	})

	t.Run("non 2xx fails", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(
			writer http.ResponseWriter,
			_ *http.Request,
		) {
			writer.WriteHeader(http.StatusServiceUnavailable)
		}))
		defer server.Close()

		require.Error(t, check(
			context.Background(),
			server.URL+"/readyz",
			500*time.Millisecond,
		))
	})

	t.Run("timeout fails", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(
			writer http.ResponseWriter,
			_ *http.Request,
		) {
			time.Sleep(50 * time.Millisecond)
			writer.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		require.Error(t, check(
			context.Background(),
			server.URL+"/readyz",
			time.Millisecond,
		))
	})
}
