package migrations_test

import (
	"context"
	"database/sql"
	"strings"
	"testing"
	"time"

	"github.com/korwlik23/go-api-starter/internal/app"
	"github.com/korwlik23/go-api-starter/internal/platform/config"
	"github.com/korwlik23/go-api-starter/internal/platform/database"
	"github.com/stretchr/testify/require"
)

const (
	m9PermissionsTable   = "permissions"
	m9MigrationFilename  = "000001_create_permissions.sql"
	m9MigrationTableName = "goose_authorization_version"
)

func TestM9(t *testing.T) {
	ctx, cfg, sqlDB := openMigrationTestDatabase(t)
	m9CleanupSchema(t, sqlDB)
	t.Cleanup(func() { m9CleanupSchema(t, sqlDB) })

	provider := newMigrationTestProvider(
		t,
		sqlDB,
		cfg,
		migrationTestFileBundle(
			t,
			"authorization",
			migrationTestDialect(t, cfg),
			m9MigrationFilename,
			false,
		),
		m9MigrationTableName,
	)

	_, err := provider.UpTo(ctx, 1)
	require.NoError(t, err)
	m9AssertPermissionsSchema(t, ctx, sqlDB, cfg)
	m9AssertVersion(t, ctx, provider, 1)
	m9AssertNoSeededPermissions(t, ctx, sqlDB)
	m9AssertPermissionsBehavior(t, ctx, sqlDB, cfg, 0)

	_, err = provider.DownTo(ctx, 0)
	require.NoError(t, err)
	require.False(t, migrationTableExists(t, ctx, sqlDB, cfg, m9PermissionsTable))
	m9AssertVersion(t, ctx, provider, 0)

	_, err = provider.UpTo(ctx, 1)
	require.NoError(t, err)
	m9AssertPermissionsSchema(t, ctx, sqlDB, cfg)
	m9AssertVersion(t, ctx, provider, 1)
	m9AssertNoSeededPermissions(t, ctx, sqlDB)
	m9AssertPermissionsBehavior(t, ctx, sqlDB, cfg, 100)
}

func TestM9Parity(t *testing.T) {
	t.Parallel()

	module := database.MigrationModule{
		ID:      "authorization",
		Bundles: strictMigrationTestBundles(t, "authorization"),
	}
	require.NoError(t, app.ValidateMigrationDialectParity(
		[]database.MigrationModule{module},
	))
}

func m9AssertVersion(
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

func m9AssertNoSeededPermissions(t *testing.T, ctx context.Context, db *sql.DB) {
	t.Helper()

	var permissionCount int
	require.NoError(t, db.QueryRowContext(
		ctx,
		"SELECT COUNT(*) FROM permissions",
	).Scan(&permissionCount))
	require.Zero(t, permissionCount, "M9 must not seed the manifest-owned permission catalog")
}

func m9AssertPermissionsSchema(
	t *testing.T,
	ctx context.Context,
	db *sql.DB,
	cfg config.DatabaseConfig,
) {
	t.Helper()

	columns := migrationColumns(t, ctx, db, cfg, m9PermissionsTable)
	require.NotEmpty(t, columns, "permissions table must exist")
	require.Len(t, columns, 9, "M9 must persist only catalog identity and policy state")
	m9AssertColumn(t, columns, "id", m3ExpectedIDType(t, cfg), "NO", false)
	m9AssertColumn(t, columns, "permission_key", m3ExpectedVarcharType(t, cfg), "NO", false)
	m9AssertColumn(t, columns, "module_id", m3ExpectedVarcharType(t, cfg), "NO", false)
	m9AssertColumn(t, columns, "scope", m3ExpectedVarcharType(t, cfg), "NO", false)
	m9AssertColumn(t, columns, "tier", m3ExpectedVarcharType(t, cfg), "NO", false)
	m9AssertColumn(t, columns, "delegable", m9ExpectedBooleanType(t, cfg), "NO", true)
	m9AssertColumn(t, columns, "active", m9ExpectedBooleanType(t, cfg), "NO", true)
	m9AssertColumn(t, columns, "created_at", m3ExpectedTimestampType(t, cfg), "NO", true)
	m9AssertColumn(t, columns, "updated_at", m3ExpectedTimestampType(t, cfg), "NO", true)
	require.Equal(t, int64(16), columns["scope"].maximumLength.Int64)
	require.Equal(t, int64(16), columns["tier"].maximumLength.Int64)
	if cfg.Driver == "mysql" {
		require.Equal(t, int64(16), columns["id"].maximumLength.Int64)
		require.Equal(t, int64(512), columns["permission_key"].maximumLength.Int64)
		require.Equal(t, int64(80), columns["module_id"].maximumLength.Int64)
	} else {
		require.Equal(t, int64(255), columns["permission_key"].maximumLength.Int64)
		require.Equal(t, int64(40), columns["module_id"].maximumLength.Int64)
	}

	require.True(t, migrationHasExactConstraintColumns(
		t,
		ctx,
		db,
		cfg,
		m9PermissionsTable,
		"PRIMARY KEY",
		[]string{"id"},
	))
	require.True(t, migrationHasExactConstraintColumns(
		t,
		ctx,
		db,
		cfg,
		m9PermissionsTable,
		"UNIQUE",
		[]string{"permission_key"},
	))
	require.Empty(t, migrationForeignKeys(t, ctx, db, cfg, m9PermissionsTable))
	require.True(t, migrationHasExactIndexColumns(
		t,
		ctx,
		db,
		cfg,
		m9PermissionsTable,
		false,
		[]string{"module_id", "active"},
	))
}

func m9AssertColumn(
	t *testing.T,
	columns map[string]migrationColumn,
	name string,
	expectedType string,
	expectedNullable string,
	requiresDefault bool,
) {
	t.Helper()

	column, exists := columns[name]
	require.True(t, exists, "required column %s is missing", name)
	require.Equal(t, expectedType, column.dataType)
	require.Equal(t, expectedNullable, column.nullable)
	hasMaterialDefault := column.defaultValue.Valid &&
		!strings.EqualFold(strings.TrimSpace(column.defaultValue.String), "NULL")
	require.Equal(t, requiresDefault, hasMaterialDefault, "unexpected default for %s", name)
}

func m9ExpectedBooleanType(t *testing.T, cfg config.DatabaseConfig) string {
	t.Helper()

	if cfg.Driver == "postgres" {
		return "boolean"
	}
	if cfg.Driver == "mysql" {
		return "tinyint"
	}
	t.Fatalf("unsupported M9 database profile")
	return ""
}

func m9CleanupSchema(t *testing.T, db *sql.DB) {
	t.Helper()

	cleanupContext, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	for _, table := range []string{m9PermissionsTable, m9MigrationTableName} {
		_, err := db.ExecContext(cleanupContext, "DROP TABLE IF EXISTS "+table)
		require.NoError(t, err)
	}
}
