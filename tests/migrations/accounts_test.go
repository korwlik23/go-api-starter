package migrations_test

import (
	"bytes"
	"context"
	"database/sql"
	"fmt"
	"testing"
	"time"

	"github.com/korwlik23/go-api-starter/internal/app"
	"github.com/korwlik23/go-api-starter/internal/platform/config"
	"github.com/korwlik23/go-api-starter/internal/platform/database"
	"github.com/stretchr/testify/require"
)

const (
	m1AccountsTable      = "accounts"
	m1MigrationFilename  = "000001_create_accounts.sql"
	m1MigrationTableName = "goose_accounts_version"
)

func TestM1(t *testing.T) {
	ctx, cfg, sqlDB := openMigrationTestDatabase(t)
	m1CleanupSchema(t, sqlDB)
	t.Cleanup(func() { m1CleanupSchema(t, sqlDB) })

	provider := newMigrationTestProvider(
		t,
		sqlDB,
		cfg,
		migrationTestFileBundle(
			t,
			"accounts",
			migrationTestDialect(t, cfg),
			m1MigrationFilename,
			false,
		),
		m1MigrationTableName,
	)

	_, err := provider.UpTo(ctx, 1)
	require.NoError(t, err)
	version, err := provider.GetDBVersion(ctx)
	require.NoError(t, err)
	require.Equal(t, int64(1), version)
	m1AssertAccountsSchema(t, ctx, sqlDB, cfg)

	_, err = provider.DownTo(ctx, 0)
	require.NoError(t, err)
	require.False(t, migrationTableExists(t, ctx, sqlDB, cfg, m1AccountsTable))
	version, err = provider.GetDBVersion(ctx)
	require.NoError(t, err)
	require.Equal(t, int64(0), version)

	_, err = provider.UpTo(ctx, 1)
	require.NoError(t, err)
	m1AssertAccountsSchema(t, ctx, sqlDB, cfg)
}

func TestM1Parity(t *testing.T) {
	t.Parallel()

	module := database.MigrationModule{
		ID:      "accounts",
		Bundles: strictMigrationTestBundles(t, "accounts"),
	}
	require.NoError(t, app.ValidateMigrationDialectParity(
		[]database.MigrationModule{module},
	))
}

func m1AssertAccountsSchema(
	t *testing.T,
	ctx context.Context,
	db *sql.DB,
	cfg config.DatabaseConfig,
) {
	t.Helper()

	columns := migrationColumns(t, ctx, db, cfg, m1AccountsTable)
	require.NotEmpty(t, columns, "accounts table must exist")
	require.Len(t, columns, 5, "M1 must create only the approved account columns")
	m1AssertRequiredColumn(t, columns, "id", m1ExpectedIDType(t, cfg), false)
	m1AssertRequiredColumn(t, columns, "slug", m1ExpectedVarcharType(t, cfg), false)
	m1AssertRequiredColumn(t, columns, "status", m1ExpectedVarcharType(t, cfg), false)
	m1AssertRequiredColumn(t, columns, "created_at", m1ExpectedTimestampType(t, cfg), true)
	m1AssertRequiredColumn(t, columns, "updated_at", m1ExpectedTimestampType(t, cfg), true)

	require.Equal(t, int64(63), columns["slug"].maximumLength.Int64)
	if cfg.Driver == "mysql" {
		require.Equal(t, int64(16), columns["id"].maximumLength.Int64)
	}
	require.True(
		t,
		migrationHasExactConstraintColumns(
			t,
			ctx,
			db,
			cfg,
			m1AccountsTable,
			"PRIMARY KEY",
			[]string{"id"},
		),
	)
	require.True(
		t,
		migrationHasExactConstraintColumns(
			t,
			ctx,
			db,
			cfg,
			m1AccountsTable,
			"UNIQUE",
			[]string{"slug"},
		),
	)

	firstID := m1AccountID(t, cfg, 1)
	secondID := m1AccountID(t, cfg, 2)
	insertSQL := "INSERT INTO accounts (id, slug, status) VALUES (" +
		migrationPlaceholders(cfg, 3) + ")"
	_, err := db.ExecContext(
		ctx,
		insertSQL,
		firstID,
		"starter-account",
		"test-status",
	)
	require.NoError(t, err)
	_, err = db.ExecContext(
		ctx,
		insertSQL,
		secondID,
		"starter-account",
		"different-status",
	)
	require.Error(t, err, "duplicate account slug must be rejected")
	_, err = db.ExecContext(
		ctx,
		insertSQL,
		firstID,
		"second-account",
		"test-status",
	)
	require.Error(t, err, "duplicate account ID must be rejected")
	_, err = db.ExecContext(
		ctx,
		"INSERT INTO accounts (slug, status) VALUES ("+
			migrationPlaceholders(cfg, 2)+")",
		"missing-id",
		"test-status",
	)
	require.Error(t, err, "account ID must not have a database default")
	_, err = db.ExecContext(
		ctx,
		"INSERT INTO accounts (id, slug) VALUES ("+
			migrationPlaceholders(cfg, 2)+")",
		secondID,
		"missing-status",
	)
	require.Error(t, err, "account status must be explicit")

	var status string
	var createdAt time.Time
	var updatedAt time.Time
	err = db.QueryRowContext(
		ctx,
		"SELECT status, created_at, updated_at FROM accounts WHERE slug = "+
			migrationPlaceholder(cfg, 1),
		"starter-account",
	).Scan(&status, &createdAt, &updatedAt)
	require.NoError(t, err)
	require.Equal(t, "test-status", status)
	require.False(t, createdAt.IsZero())
	require.False(t, updatedAt.IsZero())
}

func m1AssertRequiredColumn(
	t *testing.T,
	columns map[string]migrationColumn,
	name string,
	expectedType string,
	requiresDefault bool,
) {
	t.Helper()

	column, exists := columns[name]
	require.True(t, exists, "required column %s is missing", name)
	require.Equal(t, expectedType, column.dataType)
	require.Equal(t, "NO", column.nullable)
	require.Equal(t, requiresDefault, column.defaultValue.Valid, "unexpected default for %s", name)
}

func m1CleanupSchema(t *testing.T, db *sql.DB) {
	t.Helper()

	cleanupContext, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	for _, table := range []string{m1AccountsTable, m1MigrationTableName} {
		_, err := db.ExecContext(cleanupContext, "DROP TABLE IF EXISTS "+table)
		require.NoError(t, err)
	}
}

func m1ExpectedIDType(t *testing.T, cfg config.DatabaseConfig) string {
	t.Helper()

	if cfg.Driver == "postgres" {
		return "uuid"
	}
	if cfg.Driver == "mysql" {
		return "binary"
	}
	t.Fatalf("unsupported M1 database profile")
	return ""
}

func m1ExpectedVarcharType(t *testing.T, cfg config.DatabaseConfig) string {
	t.Helper()

	if cfg.Driver == "postgres" {
		return "character varying"
	}
	if cfg.Driver == "mysql" {
		return "varchar"
	}
	t.Fatalf("unsupported M1 database profile")
	return ""
}

func m1ExpectedTimestampType(t *testing.T, cfg config.DatabaseConfig) string {
	t.Helper()

	if cfg.Driver == "postgres" {
		return "timestamp with time zone"
	}
	if cfg.Driver == "mysql" {
		return "datetime"
	}
	t.Fatalf("unsupported M1 database profile")
	return ""
}

func m1AccountID(t *testing.T, cfg config.DatabaseConfig, seed byte) any {
	t.Helper()

	if cfg.Driver == "postgres" || cfg.Driver == "mysql" {
		if cfg.Driver == "mysql" {
			return bytes.Repeat([]byte{seed}, 16)
		}
		return fmt.Sprintf("%08x-0000-4000-8000-%012x", seed, seed)
	}
	t.Fatalf("unsupported M1 database profile")
	return nil
}
