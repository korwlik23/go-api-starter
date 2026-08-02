package http

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	revisionsapp "github.com/korwlik23/go-api-starter/internal/modules/publishing/application/revisions"
	"github.com/korwlik23/go-api-starter/internal/modules/publishing/ports"
	"github.com/stretchr/testify/require"
)

func TestM3A6RevisionHandlerRejectsMissingDependencies(t *testing.T) {
	t.Parallel()

	handler, err := NewRevisionHandler(nil, nil, nil)
	require.Nil(t, handler)
	require.ErrorIs(t, err, ErrInvalidRevisionHandler)
}

func TestM3A6RevisionHandlerRejectsUnauthenticatedList(t *testing.T) {
	t.Parallel()

	handler, err := NewRevisionHandler(&revisionReaderFake{}, &revisionManagerFake{}, &accountResolverFake{})
	require.NoError(t, err)
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/revisions", handler.List)
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/revisions?translation_id=018f1f64-7b2a-7c10-8bf2-3c40d5f67582", nil)
	router.ServeHTTP(response, request)
	require.Equal(t, http.StatusUnauthorized, response.Code)
	require.Contains(t, response.Body.String(), "authentication_required")
}

type revisionReaderFake struct{}

func (*revisionReaderFake) List(context.Context, revisionsapp.ListInput) (ports.RevisionPage, error) {
	return ports.RevisionPage{}, nil
}

type revisionManagerFake struct{}

func (*revisionManagerFake) Rollback(context.Context, revisionsapp.RollbackInput) (revisionsapp.RollbackResult, error) {
	return revisionsapp.RollbackResult{}, nil
}
