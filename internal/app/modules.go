package app

import (
	"github.com/korwlik23/go-api-starter/internal/modular"
	accountsmodule "github.com/korwlik23/go-api-starter/internal/modules/accounts"
	auditmodule "github.com/korwlik23/go-api-starter/internal/modules/audit"
	authorizationmodule "github.com/korwlik23/go-api-starter/internal/modules/authorization"
	identitymodule "github.com/korwlik23/go-api-starter/internal/modules/identity"
	localizationmodule "github.com/korwlik23/go-api-starter/internal/modules/localization"
	operationsmodule "github.com/korwlik23/go-api-starter/internal/modules/operations"
)

func Modules() (*modular.Registry, error) {
	return modular.NewRegistry([]modular.Manifest{
		identitymodule.Manifest(),
		accountsmodule.Manifest(),
		authorizationmodule.Manifest(),
		localizationmodule.Manifest(),
		auditmodule.Manifest(),
		operationsmodule.Manifest(),
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
