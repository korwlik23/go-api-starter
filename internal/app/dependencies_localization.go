package app

import (
	"errors"
	"time"

	"github.com/korwlik23/go-api-starter/internal/modules/localization/adapters/bundle"
	localizationgorm "github.com/korwlik23/go-api-starter/internal/modules/localization/adapters/gorm"
	localizationapp "github.com/korwlik23/go-api-starter/internal/modules/localization/application"
	catalogapp "github.com/korwlik23/go-api-starter/internal/modules/localization/application/catalog"
	preferenceapp "github.com/korwlik23/go-api-starter/internal/modules/localization/application/preferences"
	localizationhttp "github.com/korwlik23/go-api-starter/internal/modules/localization/transport/http"
	"github.com/korwlik23/go-api-starter/internal/platform/database"
	gormdb "gorm.io/gorm"
)

var errLocalizationComposition = errors.New("app: localization composition failed")

type localizationRuntime struct {
	locales      *localizationhttp.LocaleHandler
	management   *localizationhttp.LocaleManagementHandler
	catalog      *localizationhttp.CatalogHandler
	importExport *localizationhttp.ImportExportHandler
	preferences  *localizationhttp.PreferenceHandler
}

func buildLocalizationRuntime(
	db *gormdb.DB,
	authorizer systemPermissionAuthorizer,
	transactions *database.TransactionManager,
	auditor authorizationAuditAppender,
) (localizationRuntime, error) {
	if transactions == nil || auditor == nil {
		return localizationRuntime{}, errLocalizationComposition
	}
	localizationAuditor := authorizationAuditAdapter{appender: auditor}
	locales, err := localizationgorm.NewLocaleRepositoryWithDependencies(
		db, transactions, transactions,
	)
	if err != nil {
		return localizationRuntime{}, err
	}
	catalog, err := localizationgorm.NewCatalogRepositoryWithDependencies(
		db, time.Now, transactions, transactions,
	)
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
			Authorizer:   authorizer,
			Repository:   locales,
			NewID:        newRuntimeID,
			Now:          time.Now,
			Transactions: transactions,
			Auditor:      localizationAuditor,
		},
	)
	if err != nil {
		return localizationRuntime{}, err
	}
	updateCatalog, err := catalogapp.NewUpdateWithDependencies(
		authorizer, catalog, catalog, transactions, localizationAuditor,
	)
	if err != nil {
		return localizationRuntime{}, err
	}
	importExportService, err := catalogapp.NewImportExport(
		authorizer, catalog, catalog, transactions, localizationAuditor,
	)
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
	importExportHandler, err := localizationhttp.NewImportExportHandler(importExportService)
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
		catalog: catalogHandler, importExport: importExportHandler, preferences: preferenceHandler,
	}, nil
}
