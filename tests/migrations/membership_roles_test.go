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
	m12MembershipRolesTable = "membership_roles"
	m12MigrationFilename    = "000004_create_membership_roles.sql"
)

func TestM12(t *testing.T) {
	ctx, cfg, sqlDB := openMigrationTestDatabase(t)
	m12CleanupSchema(t, sqlDB)
	t.Cleanup(func() { m12CleanupSchema(t, sqlDB) })

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
		),
		m9MigrationTableName,
	)

	_, err := identityProvider.UpTo(ctx, 1)
	require.NoError(t, err)
	_, err = accountsProvider.UpTo(ctx, 2)
	require.NoError(t, err)
	_, err = authorizationProvider.UpTo(ctx, 4)
	require.NoError(t, err)
	m12AssertMembershipRolesSchema(t, ctx, sqlDB, cfg)
	m10AssertVersion(t, ctx, authorizationProvider, 4)
	m12AssertNoSeededMembershipRoles(t, ctx, sqlDB)
	m12AssertMembershipRoleBehavior(t, ctx, sqlDB, cfg, 0)
	parentRowCounts := m12ParentRowCounts(t, ctx, sqlDB)

	_, err = authorizationProvider.DownTo(ctx, 3)
	require.NoError(t, err)
	require.False(t, migrationTableExists(t, ctx, sqlDB, cfg, m12MembershipRolesTable))
	for _, table := range []string{
		m11RolePermissionsTable,
		m10RolesTable,
		m9PermissionsTable,
		m2MembershipsTable,
		m1AccountsTable,
		m3UsersTable,
	} {
		require.True(t, migrationTableExists(t, ctx, sqlDB, cfg, table))
	}
	m12AssertParentRowsRemain(t, ctx, sqlDB, parentRowCounts)
	m10AssertVersion(t, ctx, authorizationProvider, 3)

	_, err = authorizationProvider.UpTo(ctx, 4)
	require.NoError(t, err)
	m12AssertMembershipRolesSchema(t, ctx, sqlDB, cfg)
	m10AssertVersion(t, ctx, authorizationProvider, 4)
	m12AssertNoSeededMembershipRoles(t, ctx, sqlDB)
	m12AssertMembershipRoleBehavior(t, ctx, sqlDB, cfg, 100)
}

func TestM12Parity(t *testing.T) {
	t.Parallel()

	module := database.MigrationModule{
		ID:      "authorization",
		Bundles: strictMigrationTestBundles(t, "authorization"),
	}
	require.NoError(t, app.ValidateMigrationDialectParity(
		[]database.MigrationModule{module},
	))
}

func m12AssertMembershipRolesSchema(
	t *testing.T,
	ctx context.Context,
	db *sql.DB,
	cfg config.DatabaseConfig,
) {
	t.Helper()

	columns := migrationColumns(t, ctx, db, cfg, m12MembershipRolesTable)
	require.NotEmpty(t, columns, "membership_roles table must exist")
	require.Len(t, columns, 4, "M12 must persist only the account-consistent pair and creation time")
	for _, column := range []string{"account_id", "membership_id", "role_id"} {
		m9AssertColumn(t, columns, column, m3ExpectedIDType(t, cfg), "NO", false)
		if cfg.Driver == "mysql" {
			require.Equal(t, int64(16), columns[column].maximumLength.Int64)
		}
	}
	m9AssertColumn(t, columns, "created_at", m3ExpectedTimestampType(t, cfg), "NO", true)

	require.True(t, migrationHasExactConstraintColumns(
		t,
		ctx,
		db,
		cfg,
		m12MembershipRolesTable,
		"PRIMARY KEY",
		[]string{"account_id", "membership_id", "role_id"},
	))
	require.True(t, migrationHasExactForeignKey(
		t,
		ctx,
		db,
		cfg,
		m12MembershipRolesTable,
		migrationForeignKey{
			localColumns:      []string{"account_id", "membership_id"},
			referencedTable:   m2MembershipsTable,
			referencedColumns: []string{"account_id", "id"},
			updateRule:        "RESTRICT",
			deleteRule:        "CASCADE",
		},
	))
	require.Len(
		t,
		migrationForeignKeys(t, ctx, db, cfg, m12MembershipRolesTable),
		2,
		"M12 must use exactly the two account-consistency foreign keys",
	)
	require.True(t, migrationHasExactForeignKey(
		t,
		ctx,
		db,
		cfg,
		m12MembershipRolesTable,
		migrationForeignKey{
			localColumns:      []string{"account_id", "role_id"},
			referencedTable:   m10RolesTable,
			referencedColumns: []string{"account_id", "id"},
			updateRule:        m10ExpectedRestrictiveRule(cfg),
			deleteRule:        m10ExpectedRestrictiveRule(cfg),
		},
	))
	require.True(t, migrationHasExactIndexColumns(
		t,
		ctx,
		db,
		cfg,
		m12MembershipRolesTable,
		false,
		[]string{"account_id", "role_id"},
	))
	require.True(t, migrationHasExactIndexColumns(
		t,
		ctx,
		db,
		cfg,
		m12MembershipRolesTable,
		false,
		[]string{"role_id", "account_id", "membership_id"},
	))
}

func m12AssertNoSeededMembershipRoles(t *testing.T, ctx context.Context, db *sql.DB) {
	t.Helper()

	var assignmentCount int
	require.NoError(t, db.QueryRowContext(
		ctx,
		"SELECT COUNT(*) FROM membership_roles",
	).Scan(&assignmentCount))
	require.Zero(t, assignmentCount, "M12 must not seed role assignments")
}

func m12ParentRowCounts(t *testing.T, ctx context.Context, db *sql.DB) map[string]int {
	t.Helper()

	counts := make(map[string]int, 4)
	for _, table := range []string{m10RolesTable, m2MembershipsTable, m1AccountsTable, m3UsersTable} {
		var rowCount int
		require.NoError(t, db.QueryRowContext(
			ctx,
			"SELECT COUNT(*) FROM "+table,
		).Scan(&rowCount))
		counts[table] = rowCount
	}
	return counts
}

func m12AssertParentRowsRemain(
	t *testing.T,
	ctx context.Context,
	db *sql.DB,
	expected map[string]int,
) {
	t.Helper()

	require.NotEmpty(t, expected)
	actual := m12ParentRowCounts(t, ctx, db)
	require.Equal(t, expected, actual, "M12 down must preserve every parent row")
}

func m12CleanupSchema(t *testing.T, db *sql.DB) {
	t.Helper()

	cleanupContext, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	for _, table := range []string{
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
