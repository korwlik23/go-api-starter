package config

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestA3(t *testing.T) {
	t.Parallel()

	requiredTests := []struct {
		name  string
		key   string
		clear func(*Config)
	}{
		{
			name: "session token pepper",
			key:  "SESSION_TOKEN_PEPPER",
			clear: func(cfg *Config) {
				cfg.Session.TokenPepper = ""
			},
		},
		{
			name: "CSRF token pepper",
			key:  "CSRF_TOKEN_PEPPER",
			clear: func(cfg *Config) {
				cfg.Session.CSRFTokenPepper = ""
			},
		},
		{
			name: "TOTP encryption key",
			key:  "TOTP_ENCRYPTION_KEY_B64",
			clear: func(cfg *Config) {
				cfg.Session.TOTPEncryptionKey = ""
			},
		},
		{
			name: "database password",
			key:  "DB_PASSWORD",
			clear: func(cfg *Config) {
				cfg.Database.Password = ""
			},
		},
	}

	for _, test := range requiredTests {
		test := test
		t.Run("required/"+test.name, func(t *testing.T) {
			t.Parallel()

			cfg := validSecurityConfig()
			test.clear(&cfg)

			err := Validate(cfg)
			require.EqualError(t, err, "config: "+test.key+" is required")
		})
	}

	placeholderTests := []struct {
		name string
		key  string
		set  func(*Config, string)
	}{
		{
			name: "session token pepper",
			key:  "SESSION_TOKEN_PEPPER",
			set: func(cfg *Config, value string) {
				cfg.Session.TokenPepper = value
			},
		},
		{
			name: "CSRF token pepper",
			key:  "CSRF_TOKEN_PEPPER",
			set: func(cfg *Config, value string) {
				cfg.Session.CSRFTokenPepper = value
			},
		},
		{
			name: "TOTP encryption key",
			key:  "TOTP_ENCRYPTION_KEY_B64",
			set: func(cfg *Config, value string) {
				cfg.Session.TOTPEncryptionKey = value
			},
		},
		{
			name: "database password",
			key:  "DB_PASSWORD",
			set: func(cfg *Config, value string) {
				cfg.Database.Password = value
			},
		},
	}

	for _, test := range placeholderTests {
		test := test
		t.Run("placeholder/"+test.name, func(t *testing.T) {
			t.Parallel()

			const secretValue = "CHANGE_ME"
			cfg := validSecurityConfig()
			test.set(&cfg, secretValue)

			err := Validate(cfg)
			require.EqualError(t, err, "config: "+test.key+" must not be a placeholder")
			require.NotContains(t, err.Error(), secretValue)
		})
	}
}

func validSecurityConfig() Config {
	return Config{
		Session: SessionConfig{
			TokenPepper:       "session-token-pepper",
			CSRFTokenPepper:   "csrf-token-pepper",
			TOTPEncryptionKey: "totp-encryption-key",
		},
		Database: DatabaseConfig{
			Password: "database-password",
		},
	}
}
