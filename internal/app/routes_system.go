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
