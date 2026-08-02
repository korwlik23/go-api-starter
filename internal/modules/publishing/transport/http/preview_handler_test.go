package http

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	previewapp "github.com/korwlik23/go-api-starter/internal/modules/publishing/application/preview"
	"github.com/korwlik23/go-api-starter/internal/modules/publishing/ports"
	"github.com/stretchr/testify/require"
)

func TestM3C2PreviewHandlerRejectsMissingDependencies(t *testing.T) {
	t.Parallel()

	handler, err := NewPreviewHandler(nil, nil, nil)
	require.Nil(t, handler)
	require.ErrorIs(t, err, ErrInvalidPreviewHandler)
}

func TestM3C2PreviewExchangeUsesNoStoreAndStableNotFound(t *testing.T) {
	t.Parallel()

	handler, err := NewPreviewHandler(
		&previewIssueReaderFake{},
		&previewExchangeReaderFake{err: ports.ErrPreviewNotFound},
		&accountResolverFake{},
	)
	require.NoError(t, err)

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/public/previews/exchange", handler.Exchange)
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/public/previews/exchange", jsonBody(`{"token":"bad"}`))
	request.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(response, request)
	require.Equal(t, http.StatusNotFound, response.Code)
	require.Equal(t, "no-store", response.Header().Get("Cache-Control"))
	require.Equal(t, "noindex, nofollow", response.Header().Get("X-Robots-Tag"))
	require.Contains(t, response.Body.String(), `"code":"preview_not_found"`)
	require.NotContains(t, response.Body.String(), "bad")
}

func TestM3C2PreviewIssueRequiresAuthenticatedSession(t *testing.T) {
	t.Parallel()

	handler, err := NewPreviewHandler(
		&previewIssueReaderFake{}, &previewExchangeReaderFake{}, &accountResolverFake{},
	)
	require.NoError(t, err)
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/publishing/content/:content_id/preview", handler.Issue)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/publishing/content/018f1f64-7b2a-7c10-8bf2-3c40d5f67582/preview", nil))
	require.Equal(t, http.StatusUnauthorized, response.Code)
	require.Contains(t, response.Body.String(), `"code":"authentication_required"`)
}

type previewIssueReaderFake struct {
	result previewapp.IssueResult
	err    error
}

func (reader *previewIssueReaderFake) Execute(context.Context, previewapp.IssueInput) (previewapp.IssueResult, error) {
	return reader.result, reader.err
}

type previewExchangeReaderFake struct {
	result previewapp.ExchangeResult
	err    error
}

func (reader *previewExchangeReaderFake) Execute(context.Context, string) (previewapp.ExchangeResult, error) {
	return reader.result, reader.err
}

func jsonBody(value string) *strings.Reader { return strings.NewReader(value) }
