package modular

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestManifestValidationAcceptsCanonicalAuthority(t *testing.T) {
	t.Parallel()

	manifest := Manifest{
		ID:                   "localization",
		Tier:                 TierCore,
		RequiredCapabilities: []string{"database.sql"},
		ProvidedCapabilities: []string{"localization.catalog"},
		Dependencies:         []string{"identity"},
		Permissions: []Permission{
			{
				Key:       "localization.locales.manage.system",
				Scope:     ScopeSystem,
				Tier:      TierCore,
				Delegable: false,
			},
		},
		Routes: []Route{
			{
				Method:     "PATCH",
				Path:       "/api/v1/system/locales/:locale",
				Permission: "localization.locales.manage.system",
			},
		},
		Jobs:       []Job{{ID: "localization.catalog.compact"}},
		Migrations: []Migration{{Dialect: "postgres"}, {Dialect: "mariadb"}, {Dialect: "mysql"}},
		Health:     []HealthContributor{{ID: "localization.catalog", Critical: true}},
	}

	require.NoError(t, ValidateManifest(manifest))
}

func TestManifestValidationRejectsCatalogSpoofing(t *testing.T) {
	t.Parallel()

	tests := map[string]func(*Manifest){
		"malformed module ID": func(manifest *Manifest) {
			manifest.ID = "Localization"
		},
		"duplicate permission": func(manifest *Manifest) {
			manifest.Permissions = append(manifest.Permissions, manifest.Permissions[0])
		},
		"permission owner mismatch": func(manifest *Manifest) {
			manifest.Permissions[0].Key = "identity.locales.manage.system"
		},
		"permission scope mismatch": func(manifest *Manifest) {
			manifest.Permissions[0].Scope = ScopeAny
		},
		"permission tier mismatch": func(manifest *Manifest) {
			manifest.Permissions[0].Tier = TierOptional
		},
		"undeclared route permission": func(manifest *Manifest) {
			manifest.Routes[0].Permission = "localization.translations.update.system"
		},
		"malformed permission key": func(manifest *Manifest) {
			manifest.Permissions[0].Key = "localization.locales.manage"
		},
		"duplicate route": func(manifest *Manifest) {
			manifest.Routes = append(manifest.Routes, manifest.Routes[0])
		},
		"self dependency": func(manifest *Manifest) {
			manifest.Dependencies = []string{"localization"}
		},
		"duplicate capability": func(manifest *Manifest) {
			manifest.ProvidedCapabilities = []string{"localization.catalog", "localization.catalog"}
		},
		"duplicate migration dialect": func(manifest *Manifest) {
			manifest.Migrations = append(manifest.Migrations, Migration{Dialect: "postgres"})
		},
	}

	for name, mutate := range tests {
		name, mutate := name, mutate
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			manifest := validManifestFixture()
			mutate(&manifest)
			require.Error(t, ValidateManifest(manifest))
		})
	}
}

func validManifestFixture() Manifest {
	return Manifest{
		ID:                   "localization",
		Tier:                 TierCore,
		RequiredCapabilities: []string{"database.sql"},
		ProvidedCapabilities: []string{"localization.catalog"},
		Dependencies:         []string{"identity"},
		Permissions: []Permission{{
			Key:   "localization.locales.manage.system",
			Scope: ScopeSystem,
			Tier:  TierCore,
		}},
		Routes: []Route{{
			Method:     "PATCH",
			Path:       "/api/v1/system/locales/:locale",
			Permission: "localization.locales.manage.system",
		}},
		Jobs:       []Job{{ID: "localization.catalog.compact"}},
		Migrations: []Migration{{Dialect: "postgres"}, {Dialect: "mariadb"}, {Dialect: "mysql"}},
		Health:     []HealthContributor{{ID: "localization.catalog", Critical: true}},
	}
}
