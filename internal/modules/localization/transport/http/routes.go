package http

import "github.com/gin-gonic/gin"

type RouteHandlers struct {
	Locales     *LocaleHandler
	Management  *LocaleManagementHandler
	Catalog     *CatalogHandler
	Preferences *PreferenceHandler
}

func RegisterRoutes(router *gin.RouterGroup, handlers RouteHandlers) {
	router.GET("/locales", handlers.Locales.List)
	router.POST("/locales", handlers.Management.Create)
	router.PATCH("/locales/:locale_id", handlers.Management.UpdateMetadata)
	router.PATCH("/locales/:locale_id/status", handlers.Management.UpdateStatus)
	router.PUT("/locales/:locale_id/default", handlers.Management.SetDefault)
	router.GET("/localization/catalog", handlers.Catalog.Get)
	router.PUT("/localization/catalog", handlers.Catalog.Update)
	router.GET("/locales/preferences", handlers.Preferences.Get)
	router.PUT("/locales/preferences", handlers.Preferences.Update)
}
