package config

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type LookupFunc func(key string) (string, bool)

// Load reads configuration through lookup and converts every supported setting
// to its runtime type. Policy validation is intentionally kept separate.
func Load(lookup LookupFunc) (Config, error) {
	if lookup == nil {
		return Config{}, fmt.Errorf("config: lookup function is required")
	}

	var cfg Config
	var err error

	if cfg.App.Environment, err = environmentValue(lookup, "APP_ENV"); err != nil {
		return Config{}, err
	}
	if cfg.App.Name, err = stringValue(lookup, "APP_NAME"); err != nil {
		return Config{}, err
	}
	if cfg.HTTP.Address, err = stringValue(lookup, "HTTP_ADDRESS"); err != nil {
		return Config{}, err
	}
	if cfg.HTTP.ReadHeaderTimeout, err = durationValue(lookup, "HTTP_READ_HEADER_TIMEOUT"); err != nil {
		return Config{}, err
	}
	if cfg.HTTP.ShutdownTimeout, err = durationValue(lookup, "HTTP_SHUTDOWN_TIMEOUT"); err != nil {
		return Config{}, err
	}
	if cfg.Log.Level, err = stringValue(lookup, "LOG_LEVEL"); err != nil {
		return Config{}, err
	}
	if cfg.Log.Format, err = stringValue(lookup, "LOG_FORMAT"); err != nil {
		return Config{}, err
	}
	if cfg.Origins.Public, err = originValue(lookup, "PUBLIC_ORIGIN"); err != nil {
		return Config{}, err
	}
	if cfg.Origins.Admin, err = originValue(lookup, "ADMIN_ORIGIN"); err != nil {
		return Config{}, err
	}
	if cfg.Origins.API, err = originValue(lookup, "API_ORIGIN"); err != nil {
		return Config{}, err
	}
	if cfg.HTTP.CORSAllowedOrigins, err = originListValue(lookup, "CORS_ALLOWED_ORIGINS"); err != nil {
		return Config{}, err
	}
	if cfg.HTTP.TrustedProxyCIDRs, err = listValue(lookup, "TRUSTED_PROXY_CIDRS"); err != nil {
		return Config{}, err
	}
	if cfg.Session.Store, err = stringValue(lookup, "SESSION_STORE"); err != nil {
		return Config{}, err
	}
	if cfg.Session.CookieName, err = stringValue(lookup, "SESSION_COOKIE_NAME"); err != nil {
		return Config{}, err
	}
	if cfg.Session.CookieSecure, err = boolValue(lookup, "SESSION_COOKIE_SECURE"); err != nil {
		return Config{}, err
	}
	if cfg.Session.IdleTimeout, err = durationValue(lookup, "SESSION_IDLE_TIMEOUT"); err != nil {
		return Config{}, err
	}
	if cfg.Session.AbsoluteTimeout, err = durationValue(lookup, "SESSION_ABSOLUTE_TIMEOUT"); err != nil {
		return Config{}, err
	}
	if cfg.Session.TokenPepper, err = rawValue(lookup, "SESSION_TOKEN_PEPPER"); err != nil {
		return Config{}, err
	}
	if cfg.Session.CSRFTokenPepper, err = rawValue(lookup, "CSRF_TOKEN_PEPPER"); err != nil {
		return Config{}, err
	}
	if cfg.Session.TOTPEncryptionKey, err = rawValue(lookup, "TOTP_ENCRYPTION_KEY_B64"); err != nil {
		return Config{}, err
	}
	if cfg.Authentication.Attempts.Threshold, err = uint64Value(lookup, "AUTH_ATTEMPT_THRESHOLD"); err != nil {
		return Config{}, err
	}
	if cfg.Authentication.Attempts.Window, err = durationValue(lookup, "AUTH_ATTEMPT_WINDOW"); err != nil {
		return Config{}, err
	}
	if cfg.Authentication.Attempts.LockDuration, err = durationValue(lookup, "AUTH_ATTEMPT_LOCK_DURATION"); err != nil {
		return Config{}, err
	}
	if cfg.Authentication.Attempts.CleanupBatch, err = intValue(lookup, "AUTH_ATTEMPT_CLEANUP_BATCH"); err != nil {
		return Config{}, err
	}
	if cfg.Authentication.AttemptFingerprintKey, err = rawValue(lookup, "AUTH_ATTEMPT_FINGERPRINT_KEY"); err != nil {
		return Config{}, err
	}
	if cfg.Database.Driver, err = stringValue(lookup, "DB_DRIVER"); err != nil {
		return Config{}, err
	}
	if cfg.Database.Flavor, err = optionalStringValue(lookup, "DB_FLAVOR"); err != nil {
		return Config{}, err
	}
	if cfg.Database.Host, err = stringValue(lookup, "DB_HOST"); err != nil {
		return Config{}, err
	}
	if cfg.Database.Port, err = intValue(lookup, "DB_PORT"); err != nil {
		return Config{}, err
	}
	if cfg.Database.Name, err = stringValue(lookup, "DB_NAME"); err != nil {
		return Config{}, err
	}
	if cfg.Database.User, err = stringValue(lookup, "DB_USER"); err != nil {
		return Config{}, err
	}
	if cfg.Database.Password, err = rawValue(lookup, "DB_PASSWORD"); err != nil {
		return Config{}, err
	}
	if cfg.Database.TLSMode, err = stringValue(lookup, "DB_TLS_MODE"); err != nil {
		return Config{}, err
	}
	if cfg.Database.TimeZone, err = stringValue(lookup, "DB_TIME_ZONE"); err != nil {
		return Config{}, err
	}
	if cfg.Database.ConnectTimeout, err = durationValue(lookup, "DB_CONNECT_TIMEOUT"); err != nil {
		return Config{}, err
	}
	if cfg.Database.MigrationLockTimeout, err = durationValue(lookup, "DB_MIGRATION_LOCK_TIMEOUT"); err != nil {
		return Config{}, err
	}
	if cfg.Database.MaxOpenConnections, err = intValue(lookup, "DB_MAX_OPEN_CONNECTIONS"); err != nil {
		return Config{}, err
	}
	if cfg.Database.MaxIdleConnections, err = intValue(lookup, "DB_MAX_IDLE_CONNECTIONS"); err != nil {
		return Config{}, err
	}
	if cfg.Database.ConnectionMaxLifetime, err = durationValue(lookup, "DB_CONNECTION_MAX_LIFETIME"); err != nil {
		return Config{}, err
	}
	if cfg.Localization.InitialLocales, err = listValue(lookup, "INITIAL_LOCALES"); err != nil {
		return Config{}, err
	}
	if cfg.Localization.DefaultLocale, err = stringValue(lookup, "DEFAULT_LOCALE"); err != nil {
		return Config{}, err
	}
	if cfg.RateLimits.Baseline, err = rateLimitValue(lookup, "RATE_LIMIT_BASELINE_CAPACITY", "RATE_LIMIT_BASELINE_WINDOW"); err != nil {
		return Config{}, err
	}
	if cfg.RateLimits.Login, err = rateLimitValue(lookup, "RATE_LIMIT_LOGIN_CAPACITY", "RATE_LIMIT_LOGIN_WINDOW"); err != nil {
		return Config{}, err
	}
	if cfg.RateLimits.Recovery, err = rateLimitValue(lookup, "RATE_LIMIT_RECOVERY_CAPACITY", "RATE_LIMIT_RECOVERY_WINDOW"); err != nil {
		return Config{}, err
	}
	if cfg.RateLimits.MFA, err = rateLimitValue(lookup, "RATE_LIMIT_MFA_CAPACITY", "RATE_LIMIT_MFA_WINDOW"); err != nil {
		return Config{}, err
	}
	if cfg.Redis.Enabled, err = boolValue(lookup, "REDIS_ENABLED"); err != nil {
		return Config{}, err
	}
	if cfg.Redis.Address, err = stringValue(lookup, "REDIS_ADDRESS"); err != nil {
		return Config{}, err
	}
	if cfg.Redis.Password, err = rawValue(lookup, "REDIS_PASSWORD"); err != nil {
		return Config{}, err
	}
	if cfg.Redis.Database, err = intValue(lookup, "REDIS_DATABASE"); err != nil {
		return Config{}, err
	}

	return cfg, nil
}

func environmentValue(lookup LookupFunc, key string) (Environment, error) {
	value, err := stringValue(lookup, key)
	return Environment(value), err
}

func stringValue(lookup LookupFunc, key string) (string, error) {
	value, ok := lookup(key)
	if !ok {
		return "", missingKeyError(key)
	}
	value = strings.TrimSpace(value)
	if value == "" {
		return "", fmt.Errorf("config: %s must not be empty", key)
	}
	return value, nil
}

func optionalStringValue(lookup LookupFunc, key string) (string, error) {
	value, ok := lookup(key)
	if !ok {
		return "", missingKeyError(key)
	}
	return strings.TrimSpace(value), nil
}

func rawValue(lookup LookupFunc, key string) (string, error) {
	value, ok := lookup(key)
	if !ok {
		return "", missingKeyError(key)
	}
	return value, nil
}

func boolValue(lookup LookupFunc, key string) (bool, error) {
	value, err := stringValue(lookup, key)
	if err != nil {
		return false, err
	}
	parsed, parseErr := strconv.ParseBool(value)
	if parseErr != nil {
		return false, invalidKeyError(key, "must be a boolean")
	}
	return parsed, nil
}

func intValue(lookup LookupFunc, key string) (int, error) {
	value, err := stringValue(lookup, key)
	if err != nil {
		return 0, err
	}
	parsed, parseErr := strconv.Atoi(value)
	if parseErr != nil {
		return 0, invalidKeyError(key, "must be an integer")
	}
	return parsed, nil
}

func uint64Value(lookup LookupFunc, key string) (uint64, error) {
	value, err := stringValue(lookup, key)
	if err != nil {
		return 0, err
	}
	parsed, parseErr := strconv.ParseUint(value, 10, 64)
	if parseErr != nil {
		return 0, invalidKeyError(key, "must be a non-negative integer")
	}
	return parsed, nil
}

func durationValue(lookup LookupFunc, key string) (time.Duration, error) {
	value, err := stringValue(lookup, key)
	if err != nil {
		return 0, err
	}
	parsed, parseErr := time.ParseDuration(value)
	if parseErr != nil {
		return 0, invalidKeyError(key, "must be a duration")
	}
	return parsed, nil
}

func listValue(lookup LookupFunc, key string) ([]string, error) {
	value, ok := lookup(key)
	if !ok {
		return nil, missingKeyError(key)
	}

	items := make([]string, 0)
	for item := range strings.SplitSeq(value, ",") {
		item = strings.TrimSpace(item)
		if item != "" {
			items = append(items, item)
		}
	}
	return items, nil
}

func originListValue(lookup LookupFunc, key string) ([]url.URL, error) {
	values, err := listValue(lookup, key)
	if err != nil {
		return nil, err
	}
	if len(values) == 0 {
		return nil, fmt.Errorf("config: %s must not be empty", key)
	}

	origins := make([]url.URL, 0, len(values))
	for _, value := range values {
		origin, parseErr := parseOrigin(value)
		if parseErr != nil {
			return nil, invalidKeyError(key, "must contain valid origins")
		}
		origins = append(origins, origin)
	}
	return origins, nil
}

func originValue(lookup LookupFunc, key string) (url.URL, error) {
	value, err := stringValue(lookup, key)
	if err != nil {
		return url.URL{}, err
	}
	origin, parseErr := parseOrigin(value)
	if parseErr != nil {
		return url.URL{}, invalidKeyError(key, "must be a valid origin")
	}
	return origin, nil
}

func parseOrigin(value string) (url.URL, error) {
	parsed, err := url.Parse(value)
	if err != nil ||
		(parsed.Scheme != "http" && parsed.Scheme != "https") ||
		parsed.Host == "" ||
		parsed.User != nil ||
		parsed.Path != "" ||
		parsed.RawQuery != "" ||
		parsed.Fragment != "" {
		return url.URL{}, fmt.Errorf("invalid origin")
	}
	return *parsed, nil
}

func rateLimitValue(lookup LookupFunc, capacityKey, windowKey string) (RateLimitConfig, error) {
	capacity, err := intValue(lookup, capacityKey)
	if err != nil {
		return RateLimitConfig{}, err
	}
	window, err := durationValue(lookup, windowKey)
	if err != nil {
		return RateLimitConfig{}, err
	}
	return RateLimitConfig{Capacity: capacity, Window: window}, nil
}

func missingKeyError(key string) error {
	return fmt.Errorf("config: %s is required", key)
}

func invalidKeyError(key, reason string) error {
	return fmt.Errorf("config: %s %s", key, reason)
}
