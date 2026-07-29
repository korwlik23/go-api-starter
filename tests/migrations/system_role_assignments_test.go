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
	m13SystemRoleAssignmentsTable = "system_role_assignments"
	m13MigrationFilename          = "000005_create_system_role_assignments.sql"
)

func TestM13(t *testing.T) {
	ctx, cfg, sqlDB := openMigrationTestDatabase(t)
	m13CleanupSchema(t, sqlDB)
	t.Cleanup(func() { m13CleanupSchema(t, sqlDB) })

	identityProvider := newMigrationTestProvider(
		t,
		sqlDB,
		cfg,
		migrationTestFileBundle(
			t,
			"identity",
			migrationTestDialect(t, cfg),
			m3MigrationFilename,
			false,
		),
		m3VersionTable,
	)
	accountsProvider := newMigrationTestProvider(
		t,
		sqlDB,
		cfg,
		migrationTestFilesBundle(
			t,
			"accounts",
			migrationTestDialect(t, cfg),
			m1MigrationFilename,
			m2MigrationFilename,
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
			m12MigrationFilename,
			m13MigrationFilename,
		),
		m9MigrationTableName,
	)

	_, err := identityProvider.UpTo(ctx, 1)
	require.NoError(t, err)
	_, err = accountsProvider.UpTo(ctx, 2)
	require.NoError(t, err)
	_, err = authorizationProvider.UpTo(ctx, 5)
	require.NoError(t, err)
	m13AssertSystemRoleAssignmentsSchema(t, ctx, sqlDB, cfg)
	m10AssertVersion(t, ctx, authorizationProvider, 5)
	m13AssertNoSeededAssignments(t, ctx, sqlDB)
	m13AssertSystemRoleAssignmentBehavior(t, ctx, sqlDB, cfg, 0)
	parentRowCounts := m13ParentRowCounts(t, ctx, sqlDB)

	_, err = authorizationProvider.DownTo(ctx, 4)
	require.NoError(t, err)
	require.False(t, migrationTableExists(t, ctx, sqlDB, cfg, m13SystemRoleAssignmentsTable))
	for _, table := range []string{
		m12MembershipRolesTable,
		m11RolePermissionsTable,
		m10RolesTable,
		m9PermissionsTable,
		m2MembershipsTable,
		m1AccountsTable,
		m3UsersTable,
	} {
		require.True(t, migrationTableExists(t, ctx, sqlDB, cfg, table))
	}
	m13AssertParentRowsRemain(t, ctx, sqlDB, parentRowCounts)
	m10AssertVersion(t, ctx, authorizationProvider, 4)

	_, err = authorizationProvider.UpTo(ctx, 5)
	require.NoError(t, err)
	m13AssertSystemRoleAssignmentsSchema(t, ctx, sqlDB, cfg)
	m10AssertVersion(t, ctx, authorizationProvider, 5)
	m13AssertNoSeededAssignments(t, ctx, sqlDB)
	m13AssertSystemRoleAssignmentBehavior(t, ctx, sqlDB, cfg, 100)
}

func TestM13Parity(t *testing.T) {
	t.Parallel()

	module := database.MigrationModule{
		ID:      "authorization",
		Bundles: strictMigrationTestBundles(t, "authorization"),
	}
	require.NoError(t, app.ValidateMigrationDialectParity(
		[]database.MigrationModule{module},
	))
}

func m13AssertSystemRoleAssignmentsSchema(
	t *testing.T,
	ctx context.Context,
	db *sql.DB,
	cfg config.DatabaseConfig,
) {
	t.Helper()

	columns := migrationColumns(t, ctx, db, cfg, m13SystemRoleAssignmentsTable)
	require.NotEmpty(t, columns, "system_role_assignments table must exist")
	require.Len(t, columns, 4, "M13 must persist only the constrained user-role pair and creation time")
	for _, column := range []string{"user_id", "role_id"} {
		m9AssertColumn(t, columns, column, m3ExpectedIDType(t, cfg), "NO", false)
		if cfg.Driver == "mysql" {
			require.Equal(t, int64(16), columns[column].maximumLength.Int64)
		}
	}
	m9AssertColumn(t, columns, "role_owner_type", m3ExpectedVarcharType(t, cfg), "NO", false)
	require.Equal(t, int64(16), columns["role_owner_type"].maximumLength.Int64)
	m9AssertColumn(t, columns, "created_at", m3ExpectedTimestampType(t, cfg), "NO", true)

	require.True(t, migrationHasExactConstraintColumns(
		t,
		ctx,
		db,
		cfg,
		m13SystemRoleAssignmentsTable,
		"PRIMARY KEY",
		[]string{"user_id", "role_id"},
	))
	require.True(t, migrationHasExactForeignKey(
		t,
		ctx,
		db,
		cfg,
		m13SystemRoleAssignmentsTable,
		migrationForeignKey{
			localColumns:      []string{"user_id"},
			referencedTable:   m3UsersTable,
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
		m13SystemRoleAssignmentsTable,
		migrationForeignKey{
			localColumns:      []string{"role_owner_type", "role_id"},
			referencedTable:   m10RolesTable,
			referencedColumns: []string{"owner_type", "id"},
			updateRule:        m10ExpectedRestrictiveRule(cfg),
			deleteRule:        m10ExpectedRestrictiveRule(cfg),
		},
	))
	require.Len(
		t,
		migrationForeignKeys(t, ctx, db, cfg, m13SystemRoleAssignmentsTable),
		2,
		"M13 must use exactly the user and system-role foreign keys",
	)
	require.True(t, migrationHasExactIndexColumns(
		t,
		ctx,
		db,
		cfg,
		m13SystemRoleAssignmentsTable,
		false,
		[]string{"role_owner_type", "role_id"},
	))
	require.True(t, migrationHasExactIndexColumns(
		t,
		ctx,
		db,
		cfg,
		m13SystemRoleAssignmentsTable,
		false,
		[]string{"role_id", "user_id"},
	))
}

func m13AssertNoSeededAssignments(t *testing.T, ctx context.Context, db *sql.DB) {
	t.Helper()

	require.Zero(t, m13AssignmentCount(t, ctx, db), "M13 must not seed system authority")
}

func m13ParentRowCounts(t *testing.T, ctx context.Context, db *sql.DB) map[string]int {
	t.Helper()

	tables := []string{
		m12MembershipRolesTable,
		m11RolePermissionsTable,
		m10RolesTable,
		m9PermissionsTable,
		m2MembershipsTable,
		m1AccountsTable,
		m3UsersTable,
	}
	counts := make(map[string]int, len(tables))
	for _, table := range tables {
		var rowCount int
		require.NoError(t, db.QueryRowContext(
			ctx,
			"SELECT COUNT(*) FROM "+table,
		).Scan(&rowCount))
		counts[table] = rowCount
	}
	return counts
}

func m13AssertParentRowsRemain(
	t *testing.T,
	ctx context.Context,
	db *sql.DB,
	expected map[string]int,
) {
	t.Helper()

	require.NotEmpty(t, expected)
	require.Equal(t, expected, m13ParentRowCounts(t, ctx, db), "M13 down must preserve every parent row")
}

func m13CleanupSchema(t *testing.T, db *sql.DB) {
	t.Helper()

	cleanupContext, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	for _, table := range []string{
		m13SystemRoleAssignmentsTable,
		m12MembershipRolesTable,
		m11RolePermissionsTable,
		m10RolesTable,
		m9PermissionsTable,
		m2MembershipsTable,
		m1AccountsTable,
		m3UsersTable,
		m9MigrationTableName,
		m1MigrationTableName,
		m3VersionTable,
	} {
		_, err := db.ExecContext(cleanupContext, "DROP TABLE IF EXISTS "+table)
		require.NoError(t, err)
	}
}
