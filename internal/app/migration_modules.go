package app

import (
	accountsmodule "github.com/korwlik23/go-api-starter/internal/modules/accounts"
	auditmodule "github.com/korwlik23/go-api-starter/internal/modules/audit"
	authorizationmodule "github.com/korwlik23/go-api-starter/internal/modules/authorization"
	discoverabilitymodule "github.com/korwlik23/go-api-starter/internal/modules/discoverability"
	identitymodule "github.com/korwlik23/go-api-starter/internal/modules/identity"
	localizationmodule "github.com/korwlik23/go-api-starter/internal/modules/localization"
	mediamodule "github.com/korwlik23/go-api-starter/internal/modules/media"
	navigationmodule "github.com/korwlik23/go-api-starter/internal/modules/navigation"
	notificationsmodule "github.com/korwlik23/go-api-starter/internal/modules/notifications"
	operationsmodule "github.com/korwlik23/go-api-starter/internal/modules/operations"
	publishingmodule "github.com/korwlik23/go-api-starter/internal/modules/publishing"
	settingsmodule "github.com/korwlik23/go-api-starter/internal/modules/settings"
	"github.com/korwlik23/go-api-starter/internal/platform/database"
)

func MigrationModules() []database.MigrationModule {
	return []database.MigrationModule{
		identitymodule.MigrationModule(),
		accountsmodule.MigrationModule(),
		authorizationmodule.MigrationModule(),
		localizationmodule.MigrationModule(),
		operationsmodule.MigrationModule(),
		settingsmodule.MigrationModule(),
		notificationsmodule.MigrationModule(),
		auditmodule.MigrationModule(),
		publishingmodule.MigrationModule(),
		mediamodule.MigrationModule(),
		navigationmodule.MigrationModule(),
		discoverabilitymodule.MigrationModule(),
	}
}
