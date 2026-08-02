package gorm_test

import (
	"bytes"
	"context"
	"errors"
	"os"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/korwlik23/go-api-starter/internal/app"
	operationsgorm "github.com/korwlik23/go-api-starter/internal/modules/operations/adapters/gorm"
	operationsapp "github.com/korwlik23/go-api-starter/internal/modules/operations/application"
	"github.com/korwlik23/go-api-starter/internal/platform/config"
	"github.com/korwlik23/go-api-starter/internal/platform/database"
	"github.com/korwlik23/go-api-starter/tests/testsupport"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestModuleStateRepositoryReconcilesAtomically(t *testing.T) {
	if os.Getenv("RUN_DATABASE_INTEGRATION_TESTS") != "1" {
		t.Skip("database integration test is disabled")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	t.Cleanup(cancel)
	cfg := operationsDatabaseConfig(t)
	require.NoError(t, testsupport.RequireDisposableDatabase(
		config.Config{
			App:      config.AppConfig{Environment: config.Environment(os.Getenv("APP_ENV"))},
			Database: cfg,
		},
		os.Getenv("ALLOW_TEST_DATABASE_RESET"),
	))
	db, err := database.Open(ctx, cfg)
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, sqlDB.Close()) })

	migrator, err := database.NewMigrator(sqlDB, cfg, app.MigrationModules())
	require.NoError(t, err)
	require.NoError(t, migrator.Up(ctx))
	cleanOperationsProjection(t, ctx, db)

	repository, err := operationsgorm.NewModuleStateRepository(
		db,
		cfg.MigrationLockTimeout,
	)
	require.NoError(t, err)
	reconciler, err := operationsapp.NewReconcileModules(
		operationsapp.ReconcileDependencies{
			Repository: repository,
			NewID: func() (string, error) {
				id, idErr := uuid.NewV7()
				return id.String(), idErr
			},
		},
	)
	require.NoError(t, err)
	registry, err := app.Modules()
	require.NoError(t, err)
	enabled := map[string]bool{
		"identity":        true,
		"accounts":        true,
		"authorization":   true,
		"localization":    true,
		"audit":           true,
		"operations":      true,
		"publishing":      false,
		"media":           false,
		"navigation":      false,
		"discoverability": false,
	}

	first, err := reconciler.Execute(ctx, operationsapp.ReconcileRequest{
		Registry: registry,
		Enabled:  enabled,
	})
	require.NoError(t, err)
	require.Equal(t, int64(1), first.Revision)
	require.Equal(t, int64(1), first.CatalogEpoch)

	second, err := reconciler.Execute(ctx, operationsapp.ReconcileRequest{
		Registry:         registry,
		Enabled:          enabled,
		ExpectedRevision: first.Revision,
	})
	require.NoError(t, err)
	require.Equal(t, int64(2), second.Revision)
	require.Equal(t, first.CatalogEpoch, second.CatalogEpoch)
	currentRevision, err := repository.CurrentRevision(ctx)
	require.NoError(t, err)
	require.Equal(t, second.Revision, currentRevision)

	require.Equal(t, int64(10), tableCount(t, ctx, db, "module_states"))
	require.Equal(t, int64(35), tableCount(t, ctx, db, "permissions"))
	snapshot, err := repository.CurrentSnapshot(ctx)
	require.NoError(t, err)
	require.NoError(t, operationsapp.VerifyProjection(registry, enabled, snapshot))

	require.NoError(t, db.WithContext(ctx).Exec(
		"UPDATE module_states SET module_checksum = ? WHERE module_id = ?",
		bytes.Repeat([]byte{0xff}, 32),
		"identity",
	).Error)
	snapshot, err = repository.CurrentSnapshot(ctx)
	require.NoError(t, err)
	require.ErrorIs(
		t,
		operationsapp.VerifyProjection(registry, enabled, snapshot),
		operationsapp.ErrProjectionDrift,
	)
	repaired, err := reconciler.Execute(ctx, operationsapp.ReconcileRequest{
		Registry:         registry,
		Enabled:          enabled,
		ExpectedRevision: second.Revision,
	})
	require.NoError(t, err)

	var wait sync.WaitGroup
	wait.Add(2)
	errorsFound := make(chan error, 2)
	for range 2 {
		go func() {
			defer wait.Done()
			_, reconcileErr := reconciler.Execute(
				ctx,
				operationsapp.ReconcileRequest{
					Registry:         registry,
					Enabled:          enabled,
					ExpectedRevision: repaired.Revision,
				},
			)
			errorsFound <- reconcileErr
		}()
	}
	wait.Wait()
	close(errorsFound)

	successes := 0
	stale := 0
	for reconcileErr := range errorsFound {
		switch {
		case reconcileErr == nil:
			successes++
		case errors.Is(reconcileErr, operationsapp.ErrStaleReconcile):
			stale++
		default:
			require.NoError(t, reconcileErr)
		}
	}
	require.Equal(t, 1, successes)
	require.Equal(t, 1, stale)
}

func cleanOperationsProjection(t *testing.T, ctx context.Context, db *gorm.DB) {
	t.Helper()

	for _, statement := range []string{
		"DELETE FROM role_permissions",
		"DELETE FROM module_states",
		"DELETE FROM permissions",
	} {
		require.NoError(t, db.WithContext(ctx).Exec(statement).Error)
	}
}

func tableCount(
	t *testing.T,
	ctx context.Context,
	db *gorm.DB,
	table string,
) int64 {
	t.Helper()

	var count int64
	require.NoError(t, db.WithContext(ctx).Table(table).Count(&count).Error)
	return count
}

func operationsDatabaseConfig(t *testing.T) config.DatabaseConfig {
	t.Helper()

	return config.DatabaseConfig{
		Driver:                requiredOperationsEnv(t, "DB_DRIVER"),
		Flavor:                os.Getenv("DB_FLAVOR"),
		Host:                  requiredOperationsEnv(t, "DB_HOST"),
		Port:                  operationsEnvInt(t, "DB_PORT"),
		Name:                  requiredOperationsEnv(t, "DB_NAME"),
		User:                  requiredOperationsEnv(t, "DB_USER"),
		Password:              requiredOperationsEnv(t, "DB_PASSWORD"),
		TLSMode:               requiredOperationsEnv(t, "DB_TLS_MODE"),
		TimeZone:              requiredOperationsEnv(t, "DB_TIME_ZONE"),
		ConnectTimeout:        operationsEnvDuration(t, "DB_CONNECT_TIMEOUT"),
		MigrationLockTimeout:  operationsEnvDuration(t, "DB_MIGRATION_LOCK_TIMEOUT"),
		MaxOpenConnections:    operationsEnvInt(t, "DB_MAX_OPEN_CONNECTIONS"),
		MaxIdleConnections:    operationsEnvInt(t, "DB_MAX_IDLE_CONNECTIONS"),
		ConnectionMaxLifetime: operationsEnvDuration(t, "DB_CONNECTION_MAX_LIFETIME"),
	}
}

func requiredOperationsEnv(t *testing.T, key string) string {
	t.Helper()
	value := os.Getenv(key)
	require.NotEmpty(t, value)
	return value
}

func operationsEnvInt(t *testing.T, key string) int {
	t.Helper()
	value, err := strconv.Atoi(requiredOperationsEnv(t, key))
	require.NoError(t, err)
	return value
}

func operationsEnvDuration(t *testing.T, key string) time.Duration {
	t.Helper()
	value, err := time.ParseDuration(requiredOperationsEnv(t, key))
	require.NoError(t, err)
	return value
}
