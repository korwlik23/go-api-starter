package http

import (
	"context"
	"encoding/json"
	nethttp "net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	adminapi "github.com/korwlik23/go-api-starter/internal/generated/admin"
	"github.com/korwlik23/go-api-starter/internal/modules/operations/ports"
	"github.com/stretchr/testify/require"
)

func TestModuleHandlerListsSafeRuntimeProjection(t *testing.T) {
	t.Parallel()

	handler, err := NewModuleHandler(&moduleProjectionReaderFake{
		snapshot: ports.ReconcileSnapshot{
			Revision: 7,
			Modules: []ports.ModuleRecord{
				{
					ID:                "identity",
					Enabled:           true,
					ReconcileRevision: 7,
					ModuleChecksum:    []byte("must-not-leak"),
				},
				{
					ID:                "publishing",
					Enabled:           false,
					ReconcileRevision: 7,
					ConfigChecksum:    []byte("must-not-leak"),
				},
			},
		},
	})
	require.NoError(t, err)
	router := gin.New()
	router.GET("/api/v1/operations/modules", handler.List)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(
		nethttp.MethodGet,
		"/api/v1/operations/modules",
		nil,
	)
	router.ServeHTTP(recorder, request)

	require.Equal(t, nethttp.StatusOK, recorder.Code)
	require.Equal(t, "no-store", recorder.Header().Get("Cache-Control"))
	require.NotContains(t, recorder.Body.String(), "checksum")
	var response adminapi.ModuleListResponse
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))
	require.Equal(t, []adminapi.ModuleState{
		{Id: "identity", Enabled: true, ReconcileRevision: 7},
		{Id: "publishing", Enabled: false, ReconcileRevision: 7},
	}, response.Items)
}

type moduleProjectionReaderFake struct {
	snapshot ports.ReconcileSnapshot
	err      error
}

func (reader *moduleProjectionReaderFake) CurrentSnapshot(
	context.Context,
) (ports.ReconcileSnapshot, error) {
	return reader.snapshot, reader.err
}
