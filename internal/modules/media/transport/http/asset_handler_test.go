package http

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	mediaapp "github.com/korwlik23/go-api-starter/internal/modules/media/application"
	mediaports "github.com/korwlik23/go-api-starter/internal/modules/media/ports"
	"github.com/stretchr/testify/require"
)

func TestM3B3AssetHandlerRejectsMissingDependencies(t *testing.T) {
	t.Parallel()

	handler, err := NewAssetHandler(nil, nil)
	require.Nil(t, handler)
	require.ErrorIs(t, err, ErrInvalidAssetHandler)
}

func TestM3B3AssetHandlerRejectsUnauthenticatedList(t *testing.T) {
	t.Parallel()

	handler, err := NewAssetHandler(&assetManagerFake{}, &mediaAccountResolverFake{})
	require.NoError(t, err)
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/media/assets", handler.List)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/media/assets", nil))
	require.Equal(t, http.StatusUnauthorized, response.Code)
	require.Contains(t, response.Body.String(), "authentication_required")
}

type assetManagerFake struct{}

func (*assetManagerFake) ListAssets(context.Context, mediaapp.ListAssetsInput) (mediaports.AssetListPage, error) {
	return mediaports.AssetListPage{}, nil
}

func (*assetManagerFake) DeleteAsset(context.Context, mediaapp.DeleteAssetInput) error { return nil }
