package identity

import "github.com/korwlik23/go-api-starter/internal/modular"

func Manifest() modular.Manifest {
	return modular.Manifest{
		ID:                   "identity",
		Tier:                 modular.TierCore,
		RequiredCapabilities: []string{"database.sql"},
		ProvidedCapabilities: []string{"identity.sessions"},
		Permissions:          Permissions(),
		Routes: []modular.Route{
			{Method: "GET", Path: "/api/v1/auth/csrf"},
			{Method: "POST", Path: "/api/v1/auth/login"},
			{Method: "POST", Path: "/api/v1/auth/mfa/enrollment"},
			{Method: "POST", Path: "/api/v1/auth/mfa/enrollment/:enrollment_id/confirmation"},
			{Method: "GET", Path: "/api/v1/auth/mfa/challenge"},
			{Method: "POST", Path: "/api/v1/auth/mfa/verify"},
			{Method: "GET", Path: "/api/v1/auth/session"},
			{Method: "GET", Path: "/api/v1/auth/sessions"},
			{Method: "DELETE", Path: "/api/v1/auth/sessions/:session_id"},
			{Method: "POST", Path: "/api/v1/auth/logout"},
			{Method: "POST", Path: "/api/v1/auth/password/recovery"},
			{Method: "POST", Path: "/api/v1/auth/password/reset"},
		},
		Migrations: modular.SQLMigrations(),
		Health: []modular.HealthContributor{
			{ID: "identity.sessions", Critical: true},
		},
	}
}
