package gorm_test

import (
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/korwlik23/go-api-starter/internal/platform/config"
	"github.com/stretchr/testify/require"
)

func auditDatabaseConfig(t *testing.T) config.DatabaseConfig {
	t.Helper()

	return config.DatabaseConfig{
		Driver:                auditRequiredEnv(t, "DB_DRIVER"),
		Flavor:                os.Getenv("DB_FLAVOR"),
		Host:                  auditRequiredEnv(t, "DB_HOST"),
		Port:                  auditEnvInt(t, "DB_PORT"),
		Name:                  auditRequiredEnv(t, "DB_NAME"),
		User:                  auditRequiredEnv(t, "DB_USER"),
		Password:              auditRequiredEnv(t, "DB_PASSWORD"),
		TLSMode:               auditRequiredEnv(t, "DB_TLS_MODE"),
		TimeZone:              auditRequiredEnv(t, "DB_TIME_ZONE"),
		ConnectTimeout:        auditEnvDuration(t, "DB_CONNECT_TIMEOUT"),
		MigrationLockTimeout:  auditEnvDuration(t, "DB_MIGRATION_LOCK_TIMEOUT"),
		MaxOpenConnections:    auditEnvInt(t, "DB_MAX_OPEN_CONNECTIONS"),
		MaxIdleConnections:    auditEnvInt(t, "DB_MAX_IDLE_CONNECTIONS"),
		ConnectionMaxLifetime: auditEnvDuration(t, "DB_CONNECTION_MAX_LIFETIME"),
	}
}

func auditRequiredEnv(t *testing.T, key string) string {
	t.Helper()
	value := os.Getenv(key)
	require.NotEmpty(t, value)
	return value
}

func auditEnvInt(t *testing.T, key string) int {
	t.Helper()
	value, err := strconv.Atoi(auditRequiredEnv(t, key))
	require.NoError(t, err)
	return value
}

func auditEnvDuration(t *testing.T, key string) time.Duration {
	t.Helper()
	value, err := time.ParseDuration(auditRequiredEnv(t, key))
	require.NoError(t, err)
	return value
}
