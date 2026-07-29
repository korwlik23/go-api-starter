package config

import (
	"net/url"
	"time"
)

// Environment identifies the runtime profile selected for the application.
type Environment string

// Config contains the application configuration after environment values have
// been converted to their runtime types.
type Config struct {
	App            AppConfig
	HTTP           HTTPConfig
	Log            LogConfig
	Origins        OriginsConfig
	Session        SessionConfig
	Authentication AuthenticationConfig
	Database       DatabaseConfig
	Localization   LocalizationConfig
	RateLimits     RateLimitsConfig
	Redis          RedisConfig
}

type AppConfig struct {
	Environment Environment
	Name        string
}

type HTTPConfig struct {
	Address            string
	ReadHeaderTimeout  time.Duration
	ShutdownTimeout    time.Duration
	TrustedProxyCIDRs  []string
	CORSAllowedOrigins []url.URL
}

type LogConfig struct {
	Level  string
	Format string
}

type OriginsConfig struct {
	Public url.URL
	Admin  url.URL
	API    url.URL
}

type SessionConfig struct {
	Store                string
	CookieName           string
	CookieSecure         bool
	IdleTimeout          time.Duration
	AbsoluteTimeout      time.Duration
	TokenPepper          string
	CSRFTokenPepper      string
	TOTPActiveKeyVersion int64
	TOTPEncryptionKeys   string
}

type AuthenticationConfig struct {
	Attempts              AuthAttemptConfig
	AttemptFingerprintKey string
}

type DatabaseConfig struct {
	Driver                string
	Flavor                string
	Host                  string
	Port                  int
	Name                  string
	User                  string
	Password              string
	TLSMode               string
	TimeZone              string
	ConnectTimeout        time.Duration
	MigrationLockTimeout  time.Duration
	MaxOpenConnections    int
	MaxIdleConnections    int
	ConnectionMaxLifetime time.Duration
}

type LocalizationConfig struct {
	InitialLocales []string
	DefaultLocale  string
}

type RateLimitConfig struct {
	Capacity int
	Window   time.Duration
}

type RateLimitsConfig struct {
	Baseline RateLimitConfig
	Login    RateLimitConfig
	Recovery RateLimitConfig
	MFA      RateLimitConfig
}

type RedisConfig struct {
	Enabled  bool
	Address  string
	Password string
	Database int
}
