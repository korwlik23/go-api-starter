package authorization

import "github.com/korwlik23/go-api-starter/internal/modular"

func Manifest() modular.Manifest {
	return modular.Manifest{
		ID:                   "authorization",
		Tier:                 modular.TierCore,
		Dependencies:         []string{"accounts", "identity"},
		RequiredCapabilities: []string{"accounts.resolve", "database.sql", "identity.sessions"},
		ProvidedCapabilities: []string{"authorization.evaluate"},
		Permissions:          Permissions(),
		Routes: []modular.Route{
			{
				Method:     "GET",
				Path:       "/api/v1/authorization/effective-permissions",
				Permission: PermissionEffectiveReadOwn,
			},
			{
				Method:     "POST",
				Path:       "/api/v1/authorization/roles/:role_id/permissions",
				Permission: PermissionRolePermissionGrant,
			},
			{
				Method:     "POST",
				Path:       "/api/v1/authorization/roles/:role_id/assignments",
				Permission: PermissionRoleAssignmentAssign,
			},
			{
				Method:     "DELETE",
				Path:       "/api/v1/authorization/assignments/:assignment_id",
				Permission: PermissionRoleAssignmentRevoke,
			},
		},
		Migrations: modular.SQLMigrations(),
	}
}
