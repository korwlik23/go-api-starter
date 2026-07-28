package migrations_test

import (
	"context"
	"database/sql"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/korwlik23/go-api-starter/internal/platform/config"
	"github.com/korwlik23/go-api-starter/internal/platform/database"
	"github.com/korwlik23/go-api-starter/tests/testsupport"
	"github.com/stretchr/testify/require"
)

func openMigrationTestDatabase(
	t *testing.T,
) (context.Context, config.DatabaseConfig, *sql.DB) {
	t.Helper()

	if os.Getenv("RUN_DATABASE_INTEGRATION_TESTS") != "1" {
		t.Skip("database integration test is disabled")
	}
	cfg := migrationIntegrationConfig(t)
	require.NoError(t, testsupport.RequireDisposableDatabase(
		config.Config{
			App:      config.AppConfig{Environment: config.Environment(os.Getenv("APP_ENV"))},
			Database: cfg,
		},
		os.Getenv("ALLOW_TEST_DATABASE_RESET"),
	))

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	t.Cleanup(cancel)
	db, err := database.Open(ctx, cfg)
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, sqlDB.Close()) })
	return ctx, cfg, sqlDB
}

func migrationIntegrationConfig(t *testing.T) config.DatabaseConfig {
	t.Helper()

	return config.DatabaseConfig{
		Driver:                requiredMigrationEnvironment(t, "DB_DRIVER"),
		Flavor:                os.Getenv("DB_FLAVOR"),
		Host:                  requiredMigrationEnvironment(t, "DB_HOST"),
		Port:                  migrationEnvironmentInt(t, "DB_PORT"),
		Name:                  requiredMigrationEnvironment(t, "DB_NAME"),
		User:                  requiredMigrationEnvironment(t, "DB_USER"),
		Password:              requiredMigrationEnvironment(t, "DB_PASSWORD"),
		TLSMode:               requiredMigrationEnvironment(t, "DB_TLS_MODE"),
		TimeZone:              requiredMigrationEnvironment(t, "DB_TIME_ZONE"),
		ConnectTimeout:        migrationEnvironmentDuration(t, "DB_CONNECT_TIMEOUT"),
		MigrationLockTimeout:  migrationEnvironmentDuration(t, "DB_MIGRATION_LOCK_TIMEOUT"),
		MaxOpenConnections:    migrationEnvironmentInt(t, "DB_MAX_OPEN_CONNECTIONS"),
		MaxIdleConnections:    migrationEnvironmentInt(t, "DB_MAX_IDLE_CONNECTIONS"),
		ConnectionMaxLifetime: migrationEnvironmentDuration(t, "DB_CONNECTION_MAX_LIFETIME"),
	}
}

func requiredMigrationEnvironment(t *testing.T, key string) string {
	t.Helper()

	value, exists := os.LookupEnv(key)
	require.True(t, exists, "%s is required", key)
	require.NotEmpty(t, value, "%s must not be empty", key)
	return value
}

func migrationEnvironmentInt(t *testing.T, key string) int {
	t.Helper()

	value, err := strconv.Atoi(requiredMigrationEnvironment(t, key))
	require.NoError(t, err, "%s must be an integer", key)
	return value
}

func migrationEnvironmentDuration(t *testing.T, key string) time.Duration {
	t.Helper()

	value, err := time.ParseDuration(requiredMigrationEnvironment(t, key))
	require.NoError(t, err, "%s must be a duration", key)
	return value
}
