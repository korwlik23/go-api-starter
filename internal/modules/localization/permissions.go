package localization

import "github.com/korwlik23/go-api-starter/internal/modular"

const (
	PermissionLocalesManageSystem      = "localization.locales.manage.system"
	PermissionTranslationsUpdateSystem = "localization.translations.update.system"
)

func Permissions() []modular.Permission {
	return []modular.Permission{
		{
			Key:       PermissionLocalesManageSystem,
			Scope:     modular.ScopeSystem,
			Tier:      modular.TierCore,
			Delegable: false,
		},
		{
			Key:       PermissionTranslationsUpdateSystem,
			Scope:     modular.ScopeSystem,
			Tier:      modular.TierCore,
			Delegable: false,
		},
	}
}
