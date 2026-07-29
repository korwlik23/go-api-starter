package app

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	catalogapp "github.com/korwlik23/go-api-starter/internal/modules/localization/application/catalog"
	localizationdomain "github.com/korwlik23/go-api-starter/internal/modules/localization/domain"
	localizationhttp "github.com/korwlik23/go-api-starter/internal/modules/localization/transport/http"
	"github.com/stretchr/testify/require"
)

func TestW2CPublicLocalizationRoutes(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	locales, err := localizationhttp.NewLocaleHandler(stubLocaleLister{})
	require.NoError(t, err)
	catalog, err := localizationhttp.NewCatalogHandler(
		stubCatalogReader{},
		stubCatalogUpdater{},
	)
	require.NoError(t, err)
	router := gin.New()
	RegisterPublicSystemRoutes(
		router.Group("/api/v1"),
		locales,
		catalog,
	)

	for _, target := range []string{
		"/api/v1/locales",
		"/api/v1/localization/catalog?locale=en&category=common",
	} {
		request := httptest.NewRequest(http.MethodGet, target, nil)
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)
		require.Equal(t, http.StatusOK, response.Code, target)
	}
}

type stubLocaleLister struct{}

func (stubLocaleLister) Execute(context.Context) ([]localizationdomain.Locale, error) {
	return []localizationdomain.Locale{{
		ID: "01900000-0000-7000-8000-000000000001", Tag: "en",
		Name: "English", Direction: localizationdomain.DirectionLTR,
		Enabled: true, UserSelectable: true, Default: true, Version: 1,
	}}, nil
}

type stubCatalogReader struct{}

func (stubCatalogReader) Execute(
	context.Context,
	string,
	string,
) (catalogapp.Result, error) {
	return catalogapp.Result{
		Locale: "en", Category: "common", Version: 1,
		Entries: map[string]string{"brand": "Starter"},
	}, nil
}

type stubCatalogUpdater struct{}

func (stubCatalogUpdater) Execute(
	context.Context,
	string,
	catalogapp.UpdateCommand,
) (int64, error) {
	return 0, nil
}
