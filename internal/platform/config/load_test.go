package config

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestA2(t *testing.T) {
	t.Parallel()

	values := map[string]string{
		"APP_ENV":                      "development",
		"APP_NAME":                     "test-api",
		"HTTP_ADDRESS":                 "127.0.0.1:9090",
		"HTTP_READ_HEADER_TIMEOUT":     "7s",
		"HTTP_SHUTDOWN_TIMEOUT":        "21s",
		"LOG_LEVEL":                    "debug",
		"LOG_FORMAT":                   "json",
		"PUBLIC_ORIGIN":                "http://localhost:4321",
		"ADMIN_ORIGIN":                 "http://localhost:5173",
		"API_ORIGIN":                   "http://localhost:9090",
		"CORS_ALLOWED_ORIGINS":         "http://localhost:5173",
		"TRUSTED_PROXY_CIDRS":          "",
		"SESSION_STORE":                "database",
		"SESSION_COOKIE_NAME":          "test_session",
		"SESSION_COOKIE_SECURE":        "false",
		"SESSION_IDLE_TIMEOUT":         "30m",
		"SESSION_ABSOLUTE_TIMEOUT":     "12h",
		"SESSION_TOKEN_PEPPER":         "test-session-pepper",
		"CSRF_TOKEN_PEPPER":            "test-csrf-pepper",
		"TOTP_ENCRYPTION_KEY_B64":      "test-totp-key",
		"DB_DRIVER":                    "postgres",
		"DB_FLAVOR":                    "",
		"DB_HOST":                      "127.0.0.1",
		"DB_PORT":                      "5432",
		"DB_NAME":                      "go_lang_starter_test_config",
		"DB_USER":                      "test-user",
		"DB_PASSWORD":                  "test-password",
		"DB_TLS_MODE":                  "disable",
		"DB_TIME_ZONE":                 "UTC",
		"DB_MAX_OPEN_CONNECTIONS":      "20",
		"DB_MAX_IDLE_CONNECTIONS":      "5",
		"DB_CONNECTION_MAX_LIFETIME":   "30m",
		"INITIAL_LOCALES":              "th,en,ja",
		"DEFAULT_LOCALE":               "th",
		"RATE_LIMIT_BASELINE_CAPACITY": "120",
		"RATE_LIMIT_BASELINE_WINDOW":   "1m",
		"RATE_LIMIT_LOGIN_CAPACITY":    "5",
		"RATE_LIMIT_LOGIN_WINDOW":      "1m",
		"RATE_LIMIT_RECOVERY_CAPACITY": "3",
		"RATE_LIMIT_RECOVERY_WINDOW":   "15m",
		"RATE_LIMIT_MFA_CAPACITY":      "5",
		"RATE_LIMIT_MFA_WINDOW":        "5m",
		"REDIS_ENABLED":                "false",
		"REDIS_ADDRESS":                "127.0.0.1:6379",
		"REDIS_PASSWORD":               "",
		"REDIS_DATABASE":               "0",
	}
	lookup := func(key string) (string, bool) {
		value, ok := values[key]
		return value, ok
	}

	cfg, err := Load(lookup)
	require.NoError(t, err)
	require.Equal(t, "test-api", cfg.App.Name)
	require.Equal(t, Environment("development"), cfg.App.Environment)
	require.Equal(t, "127.0.0.1:9090", cfg.HTTP.Address)
	require.Equal(t, 7*time.Second, cfg.HTTP.ReadHeaderTimeout)
	require.Equal(t, 21*time.Second, cfg.HTTP.ShutdownTimeout)
	require.Equal(t, "http://localhost:5173", cfg.Origins.Admin.String())
	require.Empty(t, cfg.HTTP.TrustedProxyCIDRs)
	require.False(t, cfg.Session.CookieSecure)
	require.Equal(t, 30*time.Minute, cfg.Session.IdleTimeout)
	require.Equal(t, "postgres", cfg.Database.Driver)
	require.Empty(t, cfg.Database.Flavor)
	require.Equal(t, []string{"th", "en", "ja"}, cfg.Localization.InitialLocales)
	require.Equal(t, "th", cfg.Localization.DefaultLocale)
	require.Equal(t, 120, cfg.RateLimits.Baseline.Capacity)
	require.Equal(t, time.Minute, cfg.RateLimits.Baseline.Window)
	require.False(t, cfg.Redis.Enabled)
	require.Equal(t, 0, cfg.Redis.Database)

	values["SESSION_TOKEN_PEPPER"] = ""
	values["CSRF_TOKEN_PEPPER"] = "  csrf-token-pepper  "
	values["TOTP_ENCRYPTION_KEY_B64"] = "  totp-encryption-key  "
	values["DB_PASSWORD"] = "  database-password  "
	values["REDIS_PASSWORD"] = "  redis-password  "

	cfg, err = Load(lookup)
	require.NoError(t, err)
	require.Empty(t, cfg.Session.TokenPepper)
	require.Equal(t, "  csrf-token-pepper  ", cfg.Session.CSRFTokenPepper)
	require.Equal(t, "  totp-encryption-key  ", cfg.Session.TOTPEncryptionKey)
	require.Equal(t, "  database-password  ", cfg.Database.Password)
	require.Equal(t, "  redis-password  ", cfg.Redis.Password)
}
