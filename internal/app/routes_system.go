package app

import (
	"github.com/gin-gonic/gin"
	localizationhttp "github.com/korwlik23/go-api-starter/internal/modules/localization/transport/http"
)

// RegisterPublicSystemRoutes exposes only localization reads that are present
// on the public and site-server OpenAPI surfaces. Mutation handlers remain
// behind authenticated composition.
func RegisterPublicSystemRoutes(
	router *gin.RouterGroup,
	locales *localizationhttp.LocaleHandler,
	catalog *localizationhttp.CatalogHandler,
) {
	router.GET("/locales", locales.List)
	router.GET("/localization/catalog", catalog.Get)
}

func RegisterProtectedSystemRoutes(
	router *gin.RouterGroup,
	management *localizationhttp.LocaleManagementHandler,
	catalog *localizationhttp.CatalogHandler,
	preferences *localizationhttp.PreferenceHandler,
	importExport ...*localizationhttp.ImportExportHandler,
) {
	router.POST("/locales", management.Create)
	router.PATCH("/locales/:locale_id", management.UpdateMetadata)
	router.PATCH("/locales/:locale_id/status", management.UpdateStatus)
	router.PUT("/locales/:locale_id/default", management.SetDefault)
	router.PUT("/localization/catalog", catalog.Update)
	if len(importExport) > 0 && importExport[0] != nil {
		router.GET("/localization/catalog/export", importExport[0].Export)
		router.POST("/localization/catalog/import", importExport[0].Import)
	}
	router.GET("/locales/preferences", preferences.Get)
	router.PUT("/locales/preferences", preferences.Update)
}
