package config

import (
	"bytes"
	"encoding/base64"
	"testing"
	"time"

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

	bundledExampleTests := []struct {
		name  string
		key   string
		value string
		set   func(*Config, string)
	}{
		{
			name:  "session token pepper",
			key:   "SESSION_TOKEN_PEPPER",
			value: "development-only-session-token-pepper-not-for-production",
			set: func(cfg *Config, value string) {
				cfg.Session.TokenPepper = value
			},
		},
		{
			name:  "CSRF token pepper",
			key:   "CSRF_TOKEN_PEPPER",
			value: "development-only-csrf-token-pepper-not-for-production",
			set: func(cfg *Config, value string) {
				cfg.Session.CSRFTokenPepper = value
			},
		},
		{
			name:  "TOTP encryption key",
			key:   "TOTP_ENCRYPTION_KEY_B64",
			value: bundledTOTPFixture(),
			set: func(cfg *Config, value string) {
				cfg.Session.TOTPEncryptionKey = value
			},
		},
		{
			name:  "database password",
			key:   "DB_PASSWORD",
			value: "development-only-database-password",
			set: func(cfg *Config, value string) {
				cfg.Database.Password = value
			},
		},
	}

	for _, test := range bundledExampleTests {
		test := test
		t.Run("production example/"+test.name, func(t *testing.T) {
			t.Parallel()

			cfg := validSecurityConfig()
			cfg.App.Environment = Environment("production")
			test.set(&cfg, test.value)

			err := Validate(cfg)
			require.EqualError(
				t,
				err,
				"config: "+test.key+" must not use a bundled example value outside development or test",
			)
			require.NotContains(t, err.Error(), test.value)
		})
	}

	t.Run("development permits bundled examples", func(t *testing.T) {
		t.Parallel()

		cfg := validSecurityConfig()
		cfg.App.Environment = Environment("development")
		for _, test := range bundledExampleTests {
			test.set(&cfg, test.value)
		}

		require.NoError(t, Validate(cfg))
	})
}

func validSecurityConfig() Config {
	return Config{
		Session: SessionConfig{
			TokenPepper:       "session-token-pepper",
			CSRFTokenPepper:   "csrf-token-pepper",
			TOTPEncryptionKey: "totp-encryption-key",
		},
		Database: DatabaseConfig{
			Driver:                "postgres",
			Host:                  "database.internal",
			Port:                  5432,
			Name:                  "go_lang_starter_test",
			User:                  "go_lang_starter_test",
			Password:              "database-password",
			TLSMode:               "disable",
			TimeZone:              "UTC",
			ConnectTimeout:        5 * time.Second,
			MaxOpenConnections:    20,
			MaxIdleConnections:    5,
			ConnectionMaxLifetime: 30 * time.Minute,
		},
		RateLimits: RateLimitsConfig{
			Baseline: RateLimitConfig{Capacity: 120, Window: time.Minute},
			Login:    RateLimitConfig{Capacity: 5, Window: time.Minute},
			Recovery: RateLimitConfig{Capacity: 3, Window: 15 * time.Minute},
			MFA:      RateLimitConfig{Capacity: 5, Window: 5 * time.Minute},
		},
	}
}

func bundledTOTPFixture() string {
	return base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{0x11}, 32))
}
