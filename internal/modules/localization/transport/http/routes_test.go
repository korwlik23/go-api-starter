package http

import (
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestRegisterRoutesExposesLocalizationContract(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()

	RegisterRoutes(router.Group("/api/v1"), RouteHandlers{
		Locales:     &LocaleHandler{},
		Management:  &LocaleManagementHandler{},
		Catalog:     &CatalogHandler{},
		Preferences: &PreferenceHandler{},
	})

	routes := make(map[string]struct{})
	for _, route := range router.Routes() {
		routes[route.Method+" "+route.Path] = struct{}{}
	}
	for _, expected := range []string{
		"GET /api/v1/locales",
		"POST /api/v1/locales",
		"PATCH /api/v1/locales/:locale_id",
		"PATCH /api/v1/locales/:locale_id/status",
		"PUT /api/v1/locales/:locale_id/default",
		"GET /api/v1/localization/catalog",
		"PUT /api/v1/localization/catalog",
		"GET /api/v1/locales/preferences",
		"PUT /api/v1/locales/preferences",
	} {
		_, exists := routes[expected]
		require.True(t, exists, expected)
	}
}
