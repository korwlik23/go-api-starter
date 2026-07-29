package migrations_test

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/korwlik23/go-api-starter/internal/app"
	"github.com/korwlik23/go-api-starter/internal/platform/config"
	"github.com/korwlik23/go-api-starter/internal/platform/database"
	"github.com/stretchr/testify/require"
)

const (
	m18ModuleStatesTable  = "module_states"
	m18MigrationFilename  = "000001_create_module_states.sql"
	m18MigrationTableName = "goose_operations_version"
)

func TestM18(t *testing.T) {
	ctx, cfg, sqlDB := openMigrationTestDatabase(t)
	m18CleanupSchema(t, sqlDB)
	t.Cleanup(func() { m18CleanupSchema(t, sqlDB) })

	provider := newMigrationTestProvider(t, sqlDB, cfg, migrationTestFileBundle(
		t, "operations", migrationTestDialect(t, cfg), m18MigrationFilename, true,
	), m18MigrationTableName)
	_, err := provider.UpTo(ctx, 1)
	require.NoError(t, err)
	m18AssertModuleStatesSchema(t, ctx, sqlDB, cfg)
	m10AssertVersion(t, ctx, provider, 1)
	m18AssertNoSeededModuleStates(t, ctx, sqlDB)
	m18AssertModuleStateBehavior(t, ctx, sqlDB, cfg, 0)
	m18AssertModuleStateConstraints(t, ctx, sqlDB, cfg, 0)
	m18AssertModuleStateState(t, ctx, sqlDB, cfg, 0)

	_, err = provider.DownTo(ctx, 0)
	require.NoError(t, err)
	require.False(t, migrationTableExists(t, ctx, sqlDB, cfg, m18ModuleStatesTable))
	m10AssertVersion(t, ctx, provider, 0)

	_, err = provider.UpTo(ctx, 1)
	require.NoError(t, err)
	m18AssertModuleStatesSchema(t, ctx, sqlDB, cfg)
	m18AssertNoSeededModuleStates(t, ctx, sqlDB)
	m18AssertModuleStateBehavior(t, ctx, sqlDB, cfg, 100)
	m18AssertModuleStateConstraints(t, ctx, sqlDB, cfg, 100)
	m18AssertModuleStateState(t, ctx, sqlDB, cfg, 100)
	m10AssertVersion(t, ctx, provider, 1)
}

func TestM18Parity(t *testing.T) {
	t.Parallel()
	require.NoError(t, app.ValidateMigrationDialectParity([]database.MigrationModule{{
		ID: "operations", Bundles: strictMigrationTestBundles(t, "operations"),
	}}))
}

func m18AssertModuleStatesSchema(t *testing.T, ctx context.Context, db *sql.DB, cfg config.DatabaseConfig) {
	t.Helper()
	columns := migrationColumns(t, ctx, db, cfg, m18ModuleStatesTable)
	require.Len(t, columns, 8, "M18 must create only the locked module state projection")
	m9AssertColumn(t, columns, "module_id", m3ExpectedVarcharType(t, cfg), "NO", false)
	m9AssertColumn(t, columns, "enabled", m9ExpectedBooleanType(t, cfg), "NO", false)
	for _, column := range []string{"module_checksum", "config_checksum", "catalog_checksum"} {
		m9AssertColumn(t, columns, column, m18ChecksumType(t, cfg), "NO", false)
	}
	m9AssertColumn(t, columns, "catalog_epoch", "bigint", "NO", false)
	m9AssertColumn(t, columns, "reconcile_revision", "bigint", "NO", false)
	m9AssertColumn(t, columns, "reconciled_at", m3ExpectedTimestampType(t, cfg), "NO", true)
	if cfg.Driver == "mysql" {
		require.Equal(t, int64(80), columns["module_id"].maximumLength.Int64)
		for _, column := range []string{"module_checksum", "config_checksum", "catalog_checksum"} {
			require.Equal(t, int64(64), columns[column].maximumLength.Int64)
		}
	} else {
		require.Equal(t, int64(40), columns["module_id"].maximumLength.Int64)
	}
	require.Equal(t, int64(6), m18TimestampPrecision(t, ctx, db, cfg, "reconciled_at"))
	require.True(t, migrationHasExactConstraintColumns(t, ctx, db, cfg, m18ModuleStatesTable, "PRIMARY KEY", []string{"module_id"}))
	require.Len(t, migrationIndexes(t, ctx, db, cfg, m18ModuleStatesTable), 1)
	require.Empty(t, migrationForeignKeys(t, ctx, db, cfg, m18ModuleStatesTable))
}

func m18ChecksumType(t *testing.T, cfg config.DatabaseConfig) string {
	t.Helper()
	if cfg.Driver == "postgres" {
		return "bytea"
	}
	if cfg.Driver == "mysql" {
		return "varbinary"
	}
	t.Fatalf("unsupported M18 database profile")
	return ""
}

func m18TimestampPrecision(t *testing.T, ctx context.Context, db *sql.DB, cfg config.DatabaseConfig, column string) int64 {
	t.Helper()
	var precision int64
	require.NoError(t, db.QueryRowContext(ctx,
		"SELECT datetime_precision FROM information_schema.columns WHERE table_schema = "+migrationPlaceholder(cfg, 1)+
			" AND table_name = "+migrationPlaceholder(cfg, 2)+" AND column_name = "+migrationPlaceholder(cfg, 3),
		migrationCurrentSchema(t, ctx, db, cfg), m18ModuleStatesTable, column,
	).Scan(&precision))
	return precision
}

func m18AssertNoSeededModuleStates(t *testing.T, ctx context.Context, db *sql.DB) {
	t.Helper()
	var count int
	require.NoError(t, db.QueryRowContext(ctx, "SELECT COUNT(*) FROM module_states").Scan(&count))
	require.Zero(t, count)
}

func m18CleanupSchema(t *testing.T, db *sql.DB) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	for _, table := range []string{m18ModuleStatesTable, m18MigrationTableName} {
		_, err := db.ExecContext(ctx, "DROP TABLE IF EXISTS "+table)
		require.NoError(t, err)
	}
}
