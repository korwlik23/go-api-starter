package database

import (
	"context"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/korwlik23/go-api-starter/internal/platform/config"
	"github.com/stretchr/testify/require"
)

func TestD2DatabaseIntegration(t *testing.T) {
	if os.Getenv("RUN_DATABASE_INTEGRATION_TESTS") != "1" {
		t.Skip("database integration test is disabled")
	}

	cfg := databaseIntegrationConfig(t)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	db, err := Open(ctx, cfg)
	require.NoError(t, err)
	require.Equal(t, expectedDialectorName(t, cfg), db.Dialector.Name())

	sqlDB, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, sqlDB.Close())
	})

	var one int
	require.NoError(t, sqlDB.QueryRowContext(ctx, "SELECT 1").Scan(&one))
	require.Equal(t, 1, one)

	if cfg.Driver == "mysql" {
		var sessionTimeZone string
		require.NoError(
			t,
			sqlDB.QueryRowContext(ctx, "SELECT @@session.time_zone").
				Scan(&sessionTimeZone),
		)
		require.Equal(t, "+00:00", sessionTimeZone)
	}

	var timestamp time.Time
	require.NoError(
		t,
		sqlDB.QueryRowContext(ctx, timestampQuery(t, cfg.Driver)).Scan(&timestamp),
	)
	require.Equal(t, cfg.TimeZone, timestamp.Location().String())
}

func databaseIntegrationConfig(t *testing.T) config.DatabaseConfig {
	t.Helper()

	port, err := strconv.Atoi(requiredIntegrationEnvironment(t, "DB_PORT"))
	require.NoError(t, err, "DB_PORT must be an integer")
	connectTimeout, err := time.ParseDuration(
		requiredIntegrationEnvironment(t, "DB_CONNECT_TIMEOUT"),
	)
	require.NoError(t, err, "DB_CONNECT_TIMEOUT must be a duration")
	connectionMaxLifetime, err := time.ParseDuration(
		requiredIntegrationEnvironment(t, "DB_CONNECTION_MAX_LIFETIME"),
	)
	require.NoError(t, err, "DB_CONNECTION_MAX_LIFETIME must be a duration")
	maxOpenConnections, err := strconv.Atoi(
		requiredIntegrationEnvironment(t, "DB_MAX_OPEN_CONNECTIONS"),
	)
	require.NoError(t, err, "DB_MAX_OPEN_CONNECTIONS must be an integer")
	maxIdleConnections, err := strconv.Atoi(
		requiredIntegrationEnvironment(t, "DB_MAX_IDLE_CONNECTIONS"),
	)
	require.NoError(t, err, "DB_MAX_IDLE_CONNECTIONS must be an integer")

	return config.DatabaseConfig{
		Driver:                requiredIntegrationEnvironment(t, "DB_DRIVER"),
		Flavor:                os.Getenv("DB_FLAVOR"),
		Host:                  requiredIntegrationEnvironment(t, "DB_HOST"),
		Port:                  port,
		Name:                  requiredIntegrationEnvironment(t, "DB_NAME"),
		User:                  requiredIntegrationEnvironment(t, "DB_USER"),
		Password:              requiredIntegrationEnvironment(t, "DB_PASSWORD"),
		TLSMode:               requiredIntegrationEnvironment(t, "DB_TLS_MODE"),
		TimeZone:              requiredIntegrationEnvironment(t, "DB_TIME_ZONE"),
		ConnectTimeout:        connectTimeout,
		MaxOpenConnections:    maxOpenConnections,
		MaxIdleConnections:    maxIdleConnections,
		ConnectionMaxLifetime: connectionMaxLifetime,
	}
}

func requiredIntegrationEnvironment(t *testing.T, key string) string {
	t.Helper()

	value, ok := os.LookupEnv(key)
	require.True(t, ok, "%s is required", key)
	require.NotEmpty(t, value, "%s must not be empty", key)
	return value
}

func expectedDialectorName(
	t *testing.T,
	cfg config.DatabaseConfig,
) string {
	t.Helper()

	switch cfg.Driver {
	case "postgres":
		return "postgres"
	case "mysql":
		return "mysql"
	default:
		t.Fatalf("unsupported integration test driver: %s", cfg.Driver)
		return ""
	}
}

func timestampQuery(t *testing.T, driver string) string {
	t.Helper()

	switch driver {
	case "postgres":
		return "SELECT TIMESTAMP '2024-01-02 03:04:05'"
	case "mysql":
		return "SELECT CAST('2024-01-02 03:04:05' AS DATETIME)"
	default:
		t.Fatalf("unsupported integration test driver: %s", driver)
		return ""
	}
}
