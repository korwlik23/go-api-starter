package app

import (
	"github.com/korwlik23/go-api-starter/internal/modular"
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
)

func Modules() (*modular.Registry, error) {
	return modular.NewRegistry([]modular.Manifest{
		identitymodule.Manifest(),
		accountsmodule.Manifest(),
		authorizationmodule.Manifest(),
		localizationmodule.Manifest(),
		auditmodule.Manifest(),
		operationsmodule.Manifest(),
		settingsmodule.Manifest(),
		notificationsmodule.Manifest(),
		publishingmodule.Manifest(),
		mediamodule.Manifest(),
		navigationmodule.Manifest(),
		discoverabilitymodule.Manifest(),
	})
}

func enabledModuleConfiguration(registry *modular.Registry) map[string]bool {
	enabled := make(map[string]bool)
	if registry == nil {
		return enabled
	}
	for _, manifest := range registry.Ordered() {
		enabled[manifest.ID] = true
	}
	return enabled
}
