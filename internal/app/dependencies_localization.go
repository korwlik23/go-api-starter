package app

import (
	"time"

	"github.com/korwlik23/go-api-starter/internal/modules/localization/adapters/bundle"
	localizationgorm "github.com/korwlik23/go-api-starter/internal/modules/localization/adapters/gorm"
	localizationapp "github.com/korwlik23/go-api-starter/internal/modules/localization/application"
	catalogapp "github.com/korwlik23/go-api-starter/internal/modules/localization/application/catalog"
	preferenceapp "github.com/korwlik23/go-api-starter/internal/modules/localization/application/preferences"
	localizationhttp "github.com/korwlik23/go-api-starter/internal/modules/localization/transport/http"
	gormdb "gorm.io/gorm"
)

type localizationRuntime struct {
	locales     *localizationhttp.LocaleHandler
	management  *localizationhttp.LocaleManagementHandler
	catalog     *localizationhttp.CatalogHandler
	preferences *localizationhttp.PreferenceHandler
}

func buildLocalizationRuntime(
	db *gormdb.DB,
	authorizer systemPermissionAuthorizer,
) (localizationRuntime, error) {
	locales, err := localizationgorm.NewLocaleRepository(db)
	if err != nil {
		return localizationRuntime{}, err
	}
	catalog, err := localizationgorm.NewCatalogRepository(db, time.Now)
	if err != nil {
		return localizationRuntime{}, err
	}
	preferences, err := localizationgorm.NewPreferenceRepository(db, time.Now)
	if err != nil {
		return localizationRuntime{}, err
	}
	listLocales, err := localizationapp.NewListLocales(locales)
	if err != nil {
		return localizationRuntime{}, err
	}
	mergeCatalog, err := catalogapp.NewMerge(catalog, bundle.Repository{})
	if err != nil {
		return localizationRuntime{}, err
	}
	manageLocales, err := localizationapp.NewManageLocales(
		localizationapp.ManageLocaleDependencies{
			Authorizer: authorizer,
			Repository: locales,
			NewID:      newRuntimeID,
			Now:        time.Now,
		},
	)
	if err != nil {
		return localizationRuntime{}, err
	}
	updateCatalog, err := catalogapp.NewUpdate(authorizer, catalog, catalog)
	if err != nil {
		return localizationRuntime{}, err
	}
	readPreferences, err := preferenceapp.NewRead(preferences, locales)
	if err != nil {
		return localizationRuntime{}, err
	}
	updatePreferences, err := preferenceapp.NewUpdate(preferences, locales)
	if err != nil {
		return localizationRuntime{}, err
	}
	localeHandler, err := localizationhttp.NewLocaleHandler(listLocales)
	if err != nil {
		return localizationRuntime{}, err
	}
	managementHandler, err := localizationhttp.NewLocaleManagementHandler(
		manageLocales,
	)
	if err != nil {
		return localizationRuntime{}, err
	}
	catalogHandler, err := localizationhttp.NewCatalogHandler(
		mergeCatalog,
		updateCatalog,
	)
	if err != nil {
		return localizationRuntime{}, err
	}
	preferenceHandler, err := localizationhttp.NewPreferenceHandler(
		readPreferences,
		updatePreferences,
	)
	if err != nil {
		return localizationRuntime{}, err
	}
	return localizationRuntime{
		locales: localeHandler, management: managementHandler,
		catalog: catalogHandler, preferences: preferenceHandler,
	}, nil
}
