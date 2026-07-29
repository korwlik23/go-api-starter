package migrations_test

import (
	"context"
	"database/sql"
	"io/fs"
	"testing"
	"time"

	"github.com/korwlik23/go-api-starter/internal/app"
	"github.com/korwlik23/go-api-starter/internal/platform/config"
	"github.com/korwlik23/go-api-starter/internal/platform/database"
	"github.com/stretchr/testify/require"
)

const (
	m10RolesTable        = "roles"
	m10MigrationFilename = "000002_create_roles.sql"
)

func TestM10(t *testing.T) {
	ctx, cfg, sqlDB := openMigrationTestDatabase(t)
	m10CleanupSchema(t, sqlDB)
	t.Cleanup(func() { m10CleanupSchema(t, sqlDB) })

	accountsProvider := newMigrationTestProvider(
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
	authorizationProvider := newMigrationTestProvider(
		t,
		sqlDB,
		cfg,
		m10AuthorizationBundle(t, cfg),
		m9MigrationTableName,
	)

	_, err := accountsProvider.UpTo(ctx, 1)
	require.NoError(t, err)
	_, err = authorizationProvider.UpTo(ctx, 2)
	require.NoError(t, err)
	m10AssertRolesSchema(t, ctx, sqlDB, cfg)
	m10AssertVersion(t, ctx, authorizationProvider, 2)
	m10AssertNoSeededRoles(t, ctx, sqlDB)
	m10AssertRolesBehavior(t, ctx, sqlDB, cfg, 0)

	_, err = authorizationProvider.DownTo(ctx, 1)
	require.NoError(t, err)
	require.False(t, migrationTableExists(t, ctx, sqlDB, cfg, m10RolesTable))
	require.True(t, migrationTableExists(t, ctx, sqlDB, cfg, m9PermissionsTable))
	require.True(t, migrationTableExists(t, ctx, sqlDB, cfg, m1AccountsTable))
	m10AssertVersion(t, ctx, authorizationProvider, 1)

	_, err = authorizationProvider.UpTo(ctx, 2)
	require.NoError(t, err)
	m10AssertRolesSchema(t, ctx, sqlDB, cfg)
	m10AssertVersion(t, ctx, authorizationProvider, 2)
	m10AssertNoSeededRoles(t, ctx, sqlDB)
	m10AssertRolesBehavior(t, ctx, sqlDB, cfg, 100)
}

func TestM10Parity(t *testing.T) {
	t.Parallel()

	module := database.MigrationModule{
		ID:      "authorization",
		Bundles: strictMigrationTestBundles(t, "authorization"),
	}
	require.NoError(t, app.ValidateMigrationDialectParity(
		[]database.MigrationModule{module},
	))
}

func m10AuthorizationBundle(t *testing.T, cfg config.DatabaseConfig) fs.FS {
	t.Helper()

	return migrationTestFilesBundleWithOptionalLast(
		t,
		"authorization",
		migrationTestDialect(t, cfg),
		m9MigrationFilename,
		m10MigrationFilename,
	)
}

func m10AssertRolesSchema(
	t *testing.T,
	ctx context.Context,
	db *sql.DB,
	cfg config.DatabaseConfig,
) {
	t.Helper()

	columns := migrationColumns(t, ctx, db, cfg, m10RolesTable)
	require.NotEmpty(t, columns, "roles table must exist")
	require.Len(t, columns, 7, "M10 must persist only role identity, ownership, label, and version")
	m9AssertColumn(t, columns, "id", m3ExpectedIDType(t, cfg), "NO", false)
	m9AssertColumn(t, columns, "owner_type", m3ExpectedVarcharType(t, cfg), "NO", false)
	m9AssertColumn(t, columns, "account_id", m3ExpectedIDType(t, cfg), "YES", false)
	m9AssertColumn(t, columns, "name", m3ExpectedVarcharType(t, cfg), "NO", false)
	m9AssertColumn(t, columns, "version", "bigint", "NO", true)
	m9AssertColumn(t, columns, "created_at", m3ExpectedTimestampType(t, cfg), "NO", true)
	m9AssertColumn(t, columns, "updated_at", m3ExpectedTimestampType(t, cfg), "NO", true)
	require.Equal(t, int64(16), columns["owner_type"].maximumLength.Int64)
	if cfg.Driver == "mysql" {
		for _, column := range []string{"id", "account_id"} {
			require.Equal(t, int64(16), columns[column].maximumLength.Int64)
		}
		require.Equal(t, int64(256), columns["name"].maximumLength.Int64)
	} else {
		require.Equal(t, int64(128), columns["name"].maximumLength.Int64)
	}

	require.True(t, migrationHasExactConstraintColumns(
		t, ctx, db, cfg, m10RolesTable, "PRIMARY KEY", []string{"id"},
	))
	require.True(t, migrationHasExactConstraintColumns(
		t, ctx, db, cfg, m10RolesTable, "UNIQUE", []string{"account_id", "id"},
	))
	require.True(t, migrationHasExactConstraintColumns(
		t, ctx, db, cfg, m10RolesTable, "UNIQUE", []string{"owner_type", "id"},
	))
	require.True(t, migrationHasExactForeignKey(
		t,
		ctx,
		db,
		cfg,
		m10RolesTable,
		migrationForeignKey{
			localColumns:      []string{"account_id"},
			referencedTable:   m1AccountsTable,
			referencedColumns: []string{"id"},
			updateRule:        m10ExpectedRestrictiveRule(cfg),
			deleteRule:        m10ExpectedRestrictiveRule(cfg),
		},
	))
}

func m10ExpectedRestrictiveRule(cfg config.DatabaseConfig) string {
	if cfg.Driver == "mysql" && cfg.Flavor == "mysql" {
		return "NO ACTION"
	}
	return "RESTRICT"
}

func m10AssertVersion(
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

func m10AssertNoSeededRoles(t *testing.T, ctx context.Context, db *sql.DB) {
	t.Helper()

	var roleCount int
	require.NoError(t, db.QueryRowContext(ctx, "SELECT COUNT(*) FROM roles").Scan(&roleCount))
	require.Zero(t, roleCount, "M10 must not seed or hardcode role records")
}

func m10CleanupSchema(t *testing.T, db *sql.DB) {
	t.Helper()

	cleanupContext, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	for _, table := range []string{
		m10RolesTable,
		m9PermissionsTable,
		m1AccountsTable,
		m9MigrationTableName,
		m1MigrationTableName,
	} {
		_, err := db.ExecContext(cleanupContext, "DROP TABLE IF EXISTS "+table)
		require.NoError(t, err)
	}
}
