package config

import (
	"net"
	"strings"
	"time"

	_ "time/tzdata"
)

// ValidateDatabase applies fail-closed policy to a database configuration.
func ValidateDatabase(cfg DatabaseConfig) error {
	switch {
	case cfg.Driver == "postgres" && cfg.Flavor == "":
	case cfg.Driver == "mysql" &&
		(cfg.Flavor == "mariadb" || cfg.Flavor == "mysql"):
	default:
		return invalidKeyError("DB_DRIVER/DB_FLAVOR", "combination is unsupported")
	}

	if strings.TrimSpace(cfg.Host) == "" {
		return missingKeyError("DB_HOST")
	}
	if !validDatabaseHost(cfg.Host) {
		return invalidKeyError(
			"DB_HOST",
			"must be a hostname or unbracketed IP address",
		)
	}
	if cfg.Port < 1 || cfg.Port > 65535 {
		return invalidKeyError("DB_PORT", "must be between 1 and 65535")
	}
	if strings.TrimSpace(cfg.Name) == "" {
		return missingKeyError("DB_NAME")
	}
	if strings.TrimSpace(cfg.User) == "" {
		return missingKeyError("DB_USER")
	}
	if strings.TrimSpace(cfg.Password) == "" {
		return missingKeyError("DB_PASSWORD")
	}
	if !supportedTLSMode(cfg.Driver, cfg.TLSMode) {
		return invalidKeyError("DB_TLS_MODE", "is unsupported for DB_DRIVER")
	}
	if _, err := time.LoadLocation(cfg.TimeZone); err != nil {
		return invalidKeyError("DB_TIME_ZONE", "must be valid")
	}
	if cfg.Driver == "mysql" && cfg.TimeZone != "UTC" {
		return invalidKeyError("DB_TIME_ZONE", "must be UTC for DB_DRIVER")
	}
	if cfg.ConnectTimeout <= 0 {
		return invalidKeyError("DB_CONNECT_TIMEOUT", "must be greater than zero")
	}
	if cfg.MigrationLockTimeout <= 0 {
		return invalidKeyError(
			"DB_MIGRATION_LOCK_TIMEOUT",
			"must be greater than zero",
		)
	}
	if cfg.MaxOpenConnections <= 0 {
		return invalidKeyError("DB_MAX_OPEN_CONNECTIONS", "must be greater than zero")
	}
	if cfg.MaxIdleConnections <= 0 {
		return invalidKeyError("DB_MAX_IDLE_CONNECTIONS", "must be greater than zero")
	}
	if cfg.MaxIdleConnections > cfg.MaxOpenConnections {
		return invalidKeyError(
			"DB_MAX_IDLE_CONNECTIONS",
			"must not exceed DB_MAX_OPEN_CONNECTIONS",
		)
	}
	if cfg.ConnectionMaxLifetime <= 0 {
		return invalidKeyError(
			"DB_CONNECTION_MAX_LIFETIME",
			"must be greater than zero",
		)
	}
	return nil
}

func validDatabaseHost(host string) bool {
	if host != strings.TrimSpace(host) ||
		strings.ContainsAny(host, " \t\r\n/\\[],") {
		return false
	}
	if net.ParseIP(host) != nil {
		return true
	}
	return !strings.Contains(host, ":")
}

func supportedTLSMode(driver, mode string) bool {
	switch driver {
	case "postgres":
		switch mode {
		case "disable", "require", "verify-ca", "verify-full":
			return true
		}
	case "mysql":
		switch mode {
		case "disable", "require", "verify-full":
			return true
		}
	}
	return false
}
