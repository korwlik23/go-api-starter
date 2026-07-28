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
	m2MembershipsTable  = "memberships"
	m2MigrationFilename = "000002_create_memberships.sql"
)

func TestM2(t *testing.T) {
	ctx, cfg, sqlDB := openMigrationTestDatabase(t)
	m2CleanupSchema(t, sqlDB)
	t.Cleanup(func() { m2CleanupSchema(t, sqlDB) })

	identityBundles := exactMigrationTestBundles(
		t,
		"identity",
		m3MigrationFilename,
	)
	accountsBundles := exactMigrationTestBundles(
		t,
		"accounts",
		m1MigrationFilename,
		m2MigrationFilename,
	)
	migrator, err := database.NewMigrator(
		sqlDB,
		cfg,
		[]database.MigrationModule{
			{
				ID:           "accounts",
				Dependencies: []string{"identity"},
				Bundles:      accountsBundles,
			},
			{ID: "identity", Bundles: identityBundles},
		},
	)
	require.NoError(t, err)
	require.NoError(t, migrator.Up(ctx))
	m2AssertMembershipsSchema(t, ctx, sqlDB, cfg)
	m2AssertMembershipBehavior(t, ctx, sqlDB, cfg, 0)

	dialect := database.MigrationDialect(migrationTestDialect(t, cfg))
	accountsProvider := newMigrationTestProvider(
		t,
		sqlDB,
		cfg,
		accountsBundles[dialect],
		m1MigrationTableName,
	)
	identityProvider := newMigrationTestProvider(
		t,
		sqlDB,
		cfg,
		identityBundles[dialect],
		m3VersionTable,
	)
	m2AssertVersion(t, ctx, accountsProvider, 2)
	m2AssertVersion(t, ctx, identityProvider, 1)

	_, err = accountsProvider.DownTo(ctx, 1)
	require.NoError(t, err)
	require.False(t, migrationTableExists(t, ctx, sqlDB, cfg, m2MembershipsTable))
	require.True(t, migrationTableExists(t, ctx, sqlDB, cfg, m1AccountsTable))
	require.True(t, migrationTableExists(t, ctx, sqlDB, cfg, m3UsersTable))
	m2AssertVersion(t, ctx, accountsProvider, 1)
	m2AssertVersion(t, ctx, identityProvider, 1)

	_, err = accountsProvider.UpTo(ctx, 2)
	require.NoError(t, err)
	m2AssertVersion(t, ctx, accountsProvider, 2)
	m2AssertMembershipsSchema(t, ctx, sqlDB, cfg)
	m2AssertMembershipBehavior(t, ctx, sqlDB, cfg, 50)
}

func TestM2Parity(t *testing.T) {
	t.Parallel()

	module := database.MigrationModule{
		ID:      "accounts",
		Bundles: strictMigrationTestBundles(t, "accounts"),
	}
	require.NoError(t, app.ValidateMigrationDialectParity(
		[]database.MigrationModule{module},
	))
}

func m2AssertVersion(
	t *testing.T,
	ctx context.Context,
	provider interface {
		GetDBVersion(context.Context) (int64, error)
	},
	expected int64,
) {
	t.Helper()

	version, err := provider.GetDBVersion(ctx)
	require.NoError(t, err)
	require.Equal(t, expected, version)
}

func m2AssertMembershipsSchema(
	t *testing.T,
	ctx context.Context,
	db *sql.DB,
	cfg config.DatabaseConfig,
) {
	t.Helper()

	columns := migrationColumns(t, ctx, db, cfg, m2MembershipsTable)
	require.NotEmpty(t, columns, "memberships table must exist")
	require.Len(t, columns, 7, "M2 must create only approved membership columns")
	m2AssertColumn(t, columns, "id", m2ExpectedIDType(t, cfg), false)
	m2AssertColumn(t, columns, "account_id", m2ExpectedIDType(t, cfg), false)
	m2AssertColumn(t, columns, "user_id", m2ExpectedIDType(t, cfg), false)
	m2AssertColumn(t, columns, "status", m2ExpectedVarcharType(t, cfg), false)
	m2AssertColumn(t, columns, "authorization_version", "bigint", true)
	m2AssertColumn(t, columns, "created_at", m2ExpectedTimestampType(t, cfg), true)
	m2AssertColumn(t, columns, "updated_at", m2ExpectedTimestampType(t, cfg), true)
	require.Equal(t, int64(32), columns["status"].maximumLength.Int64)
	if cfg.Driver == "mysql" {
		for _, column := range []string{"id", "account_id", "user_id"} {
			require.Equal(t, int64(16), columns[column].maximumLength.Int64)
		}
	}

	require.True(t, migrationHasExactConstraintColumns(
		t, ctx, db, cfg, m2MembershipsTable, "PRIMARY KEY", []string{"id"},
	))
	require.True(t, migrationHasExactConstraintColumns(
		t,
		ctx,
		db,
		cfg,
		m2MembershipsTable,
		"UNIQUE",
		[]string{"account_id", "user_id"},
	))
	require.True(t, migrationHasExactConstraintColumns(
		t,
		ctx,
		db,
		cfg,
		m2MembershipsTable,
		"UNIQUE",
		[]string{"account_id", "id"},
	))
	require.True(t, migrationHasExactForeignKey(
		t,
		ctx,
		db,
		cfg,
		m2MembershipsTable,
		migrationForeignKey{
			localColumns:      []string{"account_id"},
			referencedTable:   m1AccountsTable,
			referencedColumns: []string{"id"},
			updateRule:        "RESTRICT",
			deleteRule:        "RESTRICT",
		},
	))
	require.True(t, migrationHasExactForeignKey(
		t,
		ctx,
		db,
		cfg,
		m2MembershipsTable,
		migrationForeignKey{
			localColumns:      []string{"user_id"},
			referencedTable:   m3UsersTable,
			referencedColumns: []string{"id"},
			updateRule:        "RESTRICT",
			deleteRule:        "RESTRICT",
		},
	))
	require.True(t, migrationHasExactIndexColumns(
		t,
		ctx,
		db,
		cfg,
		m2MembershipsTable,
		false,
		[]string{"user_id"},
	))
}

func m2AssertColumn(
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

func m2CleanupSchema(t *testing.T, db *sql.DB) {
	t.Helper()

	cleanupContext, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	for _, table := range []string{
		m2MembershipsTable,
		m1AccountsTable,
		m3UsersTable,
		m1MigrationTableName,
		m3VersionTable,
	} {
		_, err := db.ExecContext(cleanupContext, "DROP TABLE IF EXISTS "+table)
		require.NoError(t, err)
	}
}

func m2ExpectedIDType(t *testing.T, cfg config.DatabaseConfig) string {
	t.Helper()

	if cfg.Driver == "postgres" {
		return "uuid"
	}
	if cfg.Driver == "mysql" {
		return "binary"
	}
	t.Fatalf("unsupported M2 database profile")
	return ""
}

func m2ExpectedVarcharType(t *testing.T, cfg config.DatabaseConfig) string {
	t.Helper()

	if cfg.Driver == "postgres" {
		return "character varying"
	}
	if cfg.Driver == "mysql" {
		return "varchar"
	}
	t.Fatalf("unsupported M2 database profile")
	return ""
}

func m2ExpectedTimestampType(t *testing.T, cfg config.DatabaseConfig) string {
	t.Helper()

	if cfg.Driver == "postgres" {
		return "timestamp with time zone"
	}
	if cfg.Driver == "mysql" {
		return "datetime"
	}
	t.Fatalf("unsupported M2 database profile")
	return ""
}
