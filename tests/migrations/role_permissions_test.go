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
	m11RolePermissionsTable = "role_permissions"
	m11MigrationFilename    = "000003_create_role_permissions.sql"
)

func TestM11(t *testing.T) {
	ctx, cfg, sqlDB := openMigrationTestDatabase(t)
	m11CleanupSchema(t, sqlDB)
	t.Cleanup(func() { m11CleanupSchema(t, sqlDB) })

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
		migrationTestFilesBundleWithOptionalLast(
			t,
			"authorization",
			migrationTestDialect(t, cfg),
			m9MigrationFilename,
			m10MigrationFilename,
			m11MigrationFilename,
		),
		m9MigrationTableName,
	)

	_, err := accountsProvider.UpTo(ctx, 1)
	require.NoError(t, err)
	_, err = authorizationProvider.UpTo(ctx, 3)
	require.NoError(t, err)
	m11AssertRolePermissionsSchema(t, ctx, sqlDB, cfg)
	m10AssertVersion(t, ctx, authorizationProvider, 3)
	m11AssertNoSeededRolePermissions(t, ctx, sqlDB)
	m11AssertRolePermissionBehavior(t, ctx, sqlDB, cfg, 0)

	_, err = authorizationProvider.DownTo(ctx, 2)
	require.NoError(t, err)
	require.False(t, migrationTableExists(t, ctx, sqlDB, cfg, m11RolePermissionsTable))
	require.True(t, migrationTableExists(t, ctx, sqlDB, cfg, m10RolesTable))
	require.True(t, migrationTableExists(t, ctx, sqlDB, cfg, m9PermissionsTable))
	require.True(t, migrationTableExists(t, ctx, sqlDB, cfg, m1AccountsTable))
	m11AssertParentControlRowsRemain(t, ctx, sqlDB)
	m10AssertVersion(t, ctx, authorizationProvider, 2)

	_, err = authorizationProvider.UpTo(ctx, 3)
	require.NoError(t, err)
	m11AssertRolePermissionsSchema(t, ctx, sqlDB, cfg)
	m10AssertVersion(t, ctx, authorizationProvider, 3)
	m11AssertNoSeededRolePermissions(t, ctx, sqlDB)
	m11AssertRolePermissionBehavior(t, ctx, sqlDB, cfg, 100)
}

func TestM11Parity(t *testing.T) {
	t.Parallel()

	module := database.MigrationModule{
		ID:      "authorization",
		Bundles: strictMigrationTestBundles(t, "authorization"),
	}
	require.NoError(t, app.ValidateMigrationDialectParity(
		[]database.MigrationModule{module},
	))
}

func m11AssertRolePermissionsSchema(
	t *testing.T,
	ctx context.Context,
	db *sql.DB,
	cfg config.DatabaseConfig,
) {
	t.Helper()

	columns := migrationColumns(t, ctx, db, cfg, m11RolePermissionsTable)
	require.NotEmpty(t, columns, "role_permissions table must exist")
	require.Len(t, columns, 3, "M11 must persist only the mapping pair and creation time")
	m9AssertColumn(t, columns, "role_id", m3ExpectedIDType(t, cfg), "NO", false)
	m9AssertColumn(t, columns, "permission_id", m3ExpectedIDType(t, cfg), "NO", false)
	m9AssertColumn(t, columns, "created_at", m3ExpectedTimestampType(t, cfg), "NO", true)
	if cfg.Driver == "mysql" {
		require.Equal(t, int64(16), columns["role_id"].maximumLength.Int64)
		require.Equal(t, int64(16), columns["permission_id"].maximumLength.Int64)
	}

	require.True(t, migrationHasExactConstraintColumns(
		t,
		ctx,
		db,
		cfg,
		m11RolePermissionsTable,
		"PRIMARY KEY",
		[]string{"role_id", "permission_id"},
	))
	require.True(t, migrationHasExactForeignKey(
		t,
		ctx,
		db,
		cfg,
		m11RolePermissionsTable,
		migrationForeignKey{
			localColumns:      []string{"role_id"},
			referencedTable:   m10RolesTable,
			referencedColumns: []string{"id"},
			updateRule:        "RESTRICT",
			deleteRule:        "CASCADE",
		},
	))
	require.True(t, migrationHasExactForeignKey(
		t,
		ctx,
		db,
		cfg,
		m11RolePermissionsTable,
		migrationForeignKey{
			localColumns:      []string{"permission_id"},
			referencedTable:   m9PermissionsTable,
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
		m11RolePermissionsTable,
		false,
		[]string{"permission_id", "role_id"},
	))
}

func m11AssertNoSeededRolePermissions(t *testing.T, ctx context.Context, db *sql.DB) {
	t.Helper()

	var mappingCount int
	require.NoError(t, db.QueryRowContext(
		ctx,
		"SELECT COUNT(*) FROM role_permissions",
	).Scan(&mappingCount))
	require.Zero(t, mappingCount, "M11 must not seed role-permission authority")
}

func m11AssertParentControlRowsRemain(t *testing.T, ctx context.Context, db *sql.DB) {
	t.Helper()

	for table, expected := range map[string]int{
		m10RolesTable:      1,
		m9PermissionsTable: 1,
		m1AccountsTable:    1,
	} {
		var rowCount int
		require.NoError(t, db.QueryRowContext(
			ctx,
			"SELECT COUNT(*) FROM "+table,
		).Scan(&rowCount))
		require.Equal(t, expected, rowCount, "M11 down must preserve %s control rows", table)
	}
}

func m11CleanupSchema(t *testing.T, db *sql.DB) {
	t.Helper()

	cleanupContext, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	for _, table := range []string{
		m11RolePermissionsTable,
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
