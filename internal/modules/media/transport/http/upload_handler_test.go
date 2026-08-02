package http

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	accountsdomain "github.com/korwlik23/go-api-starter/internal/modules/accounts/domain"
	accountports "github.com/korwlik23/go-api-starter/internal/modules/accounts/ports"
	mediaapp "github.com/korwlik23/go-api-starter/internal/modules/media/application"
	mediadomain "github.com/korwlik23/go-api-starter/internal/modules/media/domain"
	"github.com/korwlik23/go-api-starter/internal/modules/media/ports"
	"github.com/stretchr/testify/require"
)

func TestM3B3UploadHandlerRejectsMissingDependencies(t *testing.T) {
	t.Parallel()

	handler, err := NewUploadHandler(nil, nil)
	require.Nil(t, handler)
	require.ErrorIs(t, err, ErrInvalidUploadHandler)
}

func TestM3B3UploadHandlerRejectsUnauthenticatedStart(t *testing.T) {
	t.Parallel()

	handler, err := NewUploadHandler(&uploadManagerFake{}, &mediaAccountResolverFake{})
	require.NoError(t, err)
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/media/uploads", handler.Start)
	request := httptest.NewRequest(http.MethodPost, "/media/uploads", strings.NewReader("data"))
	request.Header.Set("Content-Type", "image/png")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	require.Equal(t, http.StatusUnauthorized, response.Code)
	require.Contains(t, response.Body.String(), "authentication_required")
}

func TestM3B3UploadHandlerRequiresUploadHeadersAndBoundedLength(t *testing.T) {
	t.Parallel()

	handler, err := NewUploadHandler(&uploadManagerFake{}, &mediaAccountResolverFake{})
	require.NoError(t, err)
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/media/uploads", handler.Start)
	request := httptest.NewRequest(http.MethodPost, "/media/uploads", strings.NewReader("data"))
	request.Header.Set("Content-Type", "image/png")
	request.Header.Set("Content-Length", "4")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	require.Equal(t, http.StatusUnauthorized, response.Code)
}

func TestM3B3UploadHandlerMapsConflictWithoutStorageDetails(t *testing.T) {
	t.Parallel()

	handler, err := NewUploadHandler(&uploadManagerFake{finalizeErr: ports.ErrUploadConflict}, &mediaAccountResolverFake{})
	require.NoError(t, err)
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/media/uploads/:upload_id/finalize", handler.Finalize)
	request := httptest.NewRequest(http.MethodPost, "/media/uploads/upload/finalize", strings.NewReader(`{"expected_version":1}`))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	require.Equal(t, http.StatusUnauthorized, response.Code)
}

type uploadManagerFake struct {
	finalizeErr error
}

type mediaAccountResolverFake struct{}

func (*mediaAccountResolverFake) Execute(context.Context, string) (accountports.ResolvedAccount, error) {
	return accountports.ResolvedAccount{Account: accountsdomain.Account{ID: "018f1f64-7b2a-7c10-8bf2-3c40d5f67586"}}, nil
}

func (*uploadManagerFake) StartUpload(context.Context, mediaapp.StartUploadInput) (mediadomain.Upload, error) {
	return mediadomain.Upload{}, nil
}

func (fake *uploadManagerFake) FinalizeUpload(context.Context, mediaapp.FinalizeUploadInput) (mediadomain.Asset, error) {
	return mediadomain.Asset{}, fake.finalizeErr
}
