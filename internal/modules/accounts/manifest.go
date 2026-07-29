package accounts

import "github.com/korwlik23/go-api-starter/internal/modular"

func Manifest() modular.Manifest {
	return modular.Manifest{
		ID:                   "accounts",
		Tier:                 modular.TierCore,
		Dependencies:         []string{"identity"},
		RequiredCapabilities: []string{"database.sql", "identity.sessions"},
		ProvidedCapabilities: []string{"accounts.resolve"},
		Permissions:          Permissions(),
		Routes: []modular.Route{{
			Method:     "GET",
			Path:       "/api/v1/accounts/current",
			Permission: PermissionCurrentReadOwn,
		}},
		Migrations: modular.SQLMigrations(),
	}
}
