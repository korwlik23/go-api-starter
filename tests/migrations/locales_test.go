package migrations_test

import (
	"bytes"
	"context"
	"database/sql"
	"fmt"
	"io/fs"
	"strings"
	"testing"
	"time"

	"github.com/korwlik23/go-api-starter/internal/app"
	"github.com/korwlik23/go-api-starter/internal/platform/config"
	"github.com/korwlik23/go-api-starter/internal/platform/database"
	"github.com/stretchr/testify/require"
)

const (
	m14LocalesTable       = "locales"
	m14MigrationFilename  = "000001_create_locales.sql"
	m14MigrationTableName = "goose_localization_version"
)

func TestM14(t *testing.T) {
	ctx, cfg, sqlDB := openMigrationTestDatabase(t)
	m14CleanupSchema(t, sqlDB)
	t.Cleanup(func() { m14CleanupSchema(t, sqlDB) })

	provider := newMigrationTestProvider(
		t,
		sqlDB,
		cfg,
		migrationTestFileBundle(
			t,
			"localization",
			migrationTestDialect(t, cfg),
			m14MigrationFilename,
			true,
		),
		m14MigrationTableName,
	)

	_, err := provider.UpTo(ctx, 1)
	require.NoError(t, err)
	m14AssertLocalesSchema(t, ctx, sqlDB, cfg)
	m9AssertVersion(t, ctx, provider, 1)
	m14AssertNoSeededLocales(t, ctx, sqlDB)
	m14AssertLocaleBehavior(t, ctx, sqlDB, cfg, 0)

	_, err = provider.DownTo(ctx, 0)
	require.NoError(t, err)
	require.False(t, migrationTableExists(t, ctx, sqlDB, cfg, m14LocalesTable))
	m9AssertVersion(t, ctx, provider, 0)

	_, err = provider.UpTo(ctx, 1)
	require.NoError(t, err)
	m14AssertLocalesSchema(t, ctx, sqlDB, cfg)
	m9AssertVersion(t, ctx, provider, 1)
	m14AssertNoSeededLocales(t, ctx, sqlDB)
	m14AssertLocaleBehavior(t, ctx, sqlDB, cfg, 100)
}

func TestM14Parity(t *testing.T) {
	t.Parallel()

	module := database.MigrationModule{
		ID:      "localization",
		Bundles: strictMigrationTestBundles(t, "localization"),
	}
	require.NoError(t, app.ValidateMigrationDialectParity(
		[]database.MigrationModule{module},
	))
}

func TestM14MySQLCheckBoundForeignKeyUsesImplicitRestrictiveActions(t *testing.T) {
	t.Parallel()

	bundle := migrationTestFileBundle(
		t,
		"localization",
		"mysql",
		m14MigrationFilename,
		false,
	)
	migration, err := fs.ReadFile(bundle, m14MigrationFilename)
	require.NoError(t, err)
	source := string(migration)
	require.Contains(t, source, "CONSTRAINT locales_fallback_fk FOREIGN KEY (fallback_locale_id)")
	require.NotContains(t, strings.ToUpper(source), "ON UPDATE")
	require.NotContains(t, strings.ToUpper(source), "ON DELETE")
}

func m14AssertLocalesSchema(
	t *testing.T,
	ctx context.Context,
	db *sql.DB,
	cfg config.DatabaseConfig,
) {
	t.Helper()

	columns := migrationColumns(t, ctx, db, cfg, m14LocalesTable)
	require.NotEmpty(t, columns, "locales table must exist")
	require.Len(t, columns, 11, "M14 must persist only locale identity, fallback, visibility, default, and version state")
	m9AssertColumn(t, columns, "id", m3ExpectedIDType(t, cfg), "NO", false)
	m9AssertColumn(t, columns, "tag", m3ExpectedVarcharType(t, cfg), "NO", false)
	m9AssertColumn(t, columns, "name", m3ExpectedVarcharType(t, cfg), "NO", false)
	m9AssertColumn(t, columns, "direction", m3ExpectedVarcharType(t, cfg), "NO", false)
	m9AssertColumn(t, columns, "fallback_locale_id", m3ExpectedIDType(t, cfg), "YES", false)
	m9AssertColumn(t, columns, "enabled", m9ExpectedBooleanType(t, cfg), "NO", true)
	m9AssertColumn(t, columns, "user_selectable", m9ExpectedBooleanType(t, cfg), "NO", true)
	m9AssertColumn(t, columns, "default_slot", "smallint", "YES", false)
	m9AssertColumn(t, columns, "version", "bigint", "NO", true)
	m9AssertColumn(t, columns, "created_at", m3ExpectedTimestampType(t, cfg), "NO", true)
	m9AssertColumn(t, columns, "updated_at", m3ExpectedTimestampType(t, cfg), "NO", true)
	require.Equal(t, int64(16), columns["direction"].maximumLength.Int64)
	if cfg.Driver == "mysql" {
		for _, column := range []string{"id", "fallback_locale_id"} {
			require.Equal(t, int64(16), columns[column].maximumLength.Int64)
		}
		require.Equal(t, int64(512), columns["tag"].maximumLength.Int64)
		require.Equal(t, int64(256), columns["name"].maximumLength.Int64)
	} else {
		require.Equal(t, int64(255), columns["tag"].maximumLength.Int64)
		require.Equal(t, int64(128), columns["name"].maximumLength.Int64)
	}

	require.True(t, migrationHasExactConstraintColumns(
		t,
		ctx,
		db,
		cfg,
		m14LocalesTable,
		"PRIMARY KEY",
		[]string{"id"},
	))
	require.True(t, migrationHasExactConstraintColumns(
		t,
		ctx,
		db,
		cfg,
		m14LocalesTable,
		"UNIQUE",
		[]string{"tag"},
	))
	require.True(t, migrationHasExactConstraintColumns(
		t,
		ctx,
		db,
		cfg,
		m14LocalesTable,
		"UNIQUE",
		[]string{"default_slot"},
	))
	require.Len(t, migrationForeignKeys(t, ctx, db, cfg, m14LocalesTable), 1)
	require.True(t, migrationHasExactForeignKey(
		t,
		ctx,
		db,
		cfg,
		m14LocalesTable,
		migrationForeignKey{
			localColumns:      []string{"fallback_locale_id"},
			referencedTable:   m14LocalesTable,
			referencedColumns: []string{"id"},
			updateRule:        m10ExpectedRestrictiveRule(cfg),
			deleteRule:        m10ExpectedRestrictiveRule(cfg),
		},
	))
	require.True(t, migrationHasExactIndexColumns(
		t,
		ctx,
		db,
		cfg,
		m14LocalesTable,
		false,
		[]string{"fallback_locale_id"},
	))
	require.True(t, migrationHasExactIndexColumns(
		t,
		ctx,
		db,
		cfg,
		m14LocalesTable,
		false,
		[]string{"enabled", "user_selectable", "tag"},
	))
}

func m14AssertNoSeededLocales(t *testing.T, ctx context.Context, db *sql.DB) {
	t.Helper()

	require.Zero(t, m14LocaleCount(t, ctx, db), "M14 must not seed deployment-owned bundled locales")
}

func m14LocaleCount(t *testing.T, ctx context.Context, db *sql.DB) int {
	t.Helper()

	var count int
	require.NoError(t, db.QueryRowContext(ctx, "SELECT COUNT(*) FROM locales").Scan(&count))
	return count
}

func m14CleanupSchema(t *testing.T, db *sql.DB) {
	t.Helper()

	cleanupContext, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	for _, table := range []string{m14LocalesTable, m14MigrationTableName} {
		_, err := db.ExecContext(cleanupContext, "DROP TABLE IF EXISTS "+table)
		require.NoError(t, err)
	}
}

func m14LocaleID(t *testing.T, cfg config.DatabaseConfig, seed byte) any {
	t.Helper()

	if cfg.Driver == "postgres" {
		return fmt.Sprintf("%08x-0000-4000-8000-%012x", seed, seed)
	}
	if cfg.Driver == "mysql" {
		return bytes.Repeat([]byte{seed}, 16)
	}
	t.Fatalf("unsupported M14 database profile")
	return nil
}
