package migrations_test

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/korwlik23/go-api-starter/internal/platform/config"
	"github.com/stretchr/testify/require"
)

func m13AssertRoleOwnerConsistency(
	t *testing.T,
	ctx context.Context,
	db *sql.DB,
	cfg config.DatabaseConfig,
	insertSQL string,
	fixtures m13Fixtures,
) {
	t.Helper()

	tests := []struct {
		name      string
		userID    any
		ownerType string
		roleID    any
	}{
		{
			name:      "the exact-system check rejects an otherwise matching account role",
			userID:    fixtures.userControl,
			ownerType: "account",
			roleID:    fixtures.roleAccount,
		},
		{
			name:      "the composite role FK rejects an account role labeled as system",
			userID:    fixtures.userTwo,
			ownerType: "system",
			roleID:    fixtures.roleAccount,
		},
		{
			name:      "a padded system discriminator is not equivalent authority",
			userID:    fixtures.userTwo,
			ownerType: "system ",
			roleID:    fixtures.roleTwo,
		},
		{
			name:      "an uppercase discriminator is rejected",
			userID:    fixtures.userControl,
			ownerType: "SYSTEM",
			roleID:    fixtures.roleOne,
		},
		{
			name:      "an empty discriminator is rejected",
			userID:    fixtures.userOne,
			ownerType: "",
			roleID:    fixtures.roleOther,
		},
	}
	for _, test := range tests {
		_, err := db.ExecContext(ctx, insertSQL, test.userID, test.ownerType, test.roleID)
		require.Error(t, err, test.name)
		m13RequireAssignmentCount(t, ctx, db, cfg, test.userID, test.roleID, 0)
	}
	require.Equal(t, 4, m13AssignmentCount(t, ctx, db), "rejected owner variants must not persist")
	m13AssertMySQLNonStrictOwnerBoundary(t, ctx, db, cfg, fixtures)
}

func m13AssertMySQLNonStrictOwnerBoundary(
	t *testing.T,
	ctx context.Context,
	db *sql.DB,
	cfg config.DatabaseConfig,
	fixtures m13Fixtures,
) {
	t.Helper()
	if cfg.Driver != "mysql" {
		return
	}

	connection, err := db.Conn(ctx)
	require.NoError(t, err)
	var previousSQLMode string
	require.NoError(t, connection.QueryRowContext(ctx, "SELECT @@SESSION.sql_mode").Scan(&previousSQLMode))
	require.NoError(t, m9SetSQLMode(ctx, connection, ""))
	defer func() {
		restoreContext, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		require.NoError(t, m9SetSQLMode(restoreContext, connection, previousSQLMode))
		require.NoError(t, connection.Close())
	}()

	_, err = connection.ExecContext(
		ctx,
		m13SystemRoleAssignmentInsertSQL(cfg),
		fixtures.userControl,
		"system          x",
		fixtures.roleTwo,
	)
	require.Error(t, err, "non-strict truncation must not create padded system authority")

	var inserted int
	require.NoError(t, connection.QueryRowContext(
		ctx,
		"SELECT COUNT(*) FROM system_role_assignments WHERE user_id = "+
			migrationPlaceholder(cfg, 1)+" AND role_id = "+migrationPlaceholder(cfg, 2),
		fixtures.userControl,
		fixtures.roleTwo,
	).Scan(&inserted))
	require.Zero(t, inserted)
}

func m13AssertParentUpdatesRestricted(
	t *testing.T,
	ctx context.Context,
	db *sql.DB,
	cfg config.DatabaseConfig,
	fixtures m13Fixtures,
	seedOffset byte,
) {
	t.Helper()

	tests := []struct {
		name      string
		statement string
		arguments []any
	}{
		{
			name: "mapped user identity",
			statement: "UPDATE users SET id = " + migrationPlaceholder(cfg, 1) +
				" WHERE id = " + migrationPlaceholder(cfg, 2),
			arguments: []any{m3UserID(t, cfg, 8+seedOffset), fixtures.userTwo},
		},
		{
			name: "mapped system role identity",
			statement: "UPDATE roles SET id = " + migrationPlaceholder(cfg, 1) +
				" WHERE id = " + migrationPlaceholder(cfg, 2),
			arguments: []any{m10RoleID(t, cfg, 15+seedOffset), fixtures.roleTwo},
		},
		{
			name: "mapped system role owner transition",
			statement: "UPDATE roles SET owner_type = " + migrationPlaceholder(cfg, 1) +
				", account_id = " + migrationPlaceholder(cfg, 2) +
				" WHERE id = " + migrationPlaceholder(cfg, 3),
			arguments: []any{"account", fixtures.accountOne, fixtures.roleTwo},
		},
	}
	for _, test := range tests {
		_, err := db.ExecContext(ctx, test.statement, test.arguments...)
		require.Error(t, err, "%s updates must be restricted while assigned", test.name)
	}
	m13RequireAssignmentCount(t, ctx, db, cfg, fixtures.userTwo, fixtures.roleOne, 1)
	m13RequireAssignmentCount(t, ctx, db, cfg, fixtures.userOne, fixtures.roleTwo, 1)
	require.Equal(t, 4, m13AssignmentCount(t, ctx, db))
}

func m13AssertDeletePolicies(
	t *testing.T,
	ctx context.Context,
	db *sql.DB,
	cfg config.DatabaseConfig,
	fixtures m13Fixtures,
) {
	t.Helper()

	_, err := db.ExecContext(
		ctx,
		"DELETE FROM roles WHERE id = "+migrationPlaceholder(cfg, 1),
		fixtures.roleOne,
	)
	require.Error(t, err, "assigned system roles require an explicit audited detach")
	m13RequireAssignmentCount(t, ctx, db, cfg, fixtures.userTwo, fixtures.roleOne, 1)
	_, err = db.ExecContext(
		ctx,
		"DELETE FROM roles WHERE id = "+migrationPlaceholder(cfg, 1),
		fixtures.roleControl,
	)
	require.NoError(t, err, "an unassigned system role must remain deletable")
	require.Equal(t, 4, m13AssignmentCount(t, ctx, db))

	var originalCreatedAt time.Time
	require.NoError(t, db.QueryRowContext(
		ctx,
		"SELECT created_at FROM system_role_assignments WHERE user_id = "+
			migrationPlaceholder(cfg, 1)+" AND role_id = "+migrationPlaceholder(cfg, 2),
		fixtures.userTwo,
		fixtures.roleOne,
	).Scan(&originalCreatedAt))
	_, err = db.ExecContext(
		ctx,
		"DELETE FROM users WHERE id = "+migrationPlaceholder(cfg, 1),
		fixtures.userOne,
	)
	require.NoError(t, err, "user deletion must cascade only that principal's assignments")
	require.Equal(t, 2, m13AssignmentCount(t, ctx, db))
	m13RequireAssignmentCount(t, ctx, db, cfg, fixtures.userTwo, fixtures.roleOne, 1)
	m13RequireAssignmentCount(t, ctx, db, cfg, fixtures.userControl, fixtures.roleOther, 1)
	m13RequireParentExists(t, ctx, db, cfg, m10RolesTable, fixtures.roleOne)
	m13RequireParentExists(t, ctx, db, cfg, m10RolesTable, fixtures.roleTwo)

	_, err = db.ExecContext(
		ctx,
		"DELETE FROM roles WHERE id = "+migrationPlaceholder(cfg, 1),
		fixtures.roleTwo,
	)
	require.NoError(t, err, "a role becomes deletable after its assignments are gone")
	_, err = db.ExecContext(
		ctx,
		"DELETE FROM roles WHERE id = "+migrationPlaceholder(cfg, 1),
		fixtures.roleOne,
	)
	require.Error(t, err, "another user still holds this role")

	deleteSQL := "DELETE FROM system_role_assignments WHERE user_id = " +
		migrationPlaceholder(cfg, 1) + " AND role_id = " + migrationPlaceholder(cfg, 2)
	_, err = db.ExecContext(ctx, deleteSQL, fixtures.userTwo, fixtures.roleOne)
	require.NoError(t, err)
	m13RequireParentExists(t, ctx, db, cfg, m3UsersTable, fixtures.userTwo)
	m13RequireParentExists(t, ctx, db, cfg, m10RolesTable, fixtures.roleOne)

	recreatedAt := originalCreatedAt.Add(time.Second)
	_, err = db.ExecContext(
		ctx,
		"INSERT INTO system_role_assignments "+
			"(user_id, role_owner_type, role_id, created_at) VALUES ("+
			migrationPlaceholders(cfg, 4)+")",
		fixtures.userTwo,
		"system",
		fixtures.roleOne,
		recreatedAt,
	)
	require.NoError(t, err)
	var storedCreatedAt time.Time
	require.NoError(t, db.QueryRowContext(
		ctx,
		"SELECT created_at FROM system_role_assignments WHERE user_id = "+
			migrationPlaceholder(cfg, 1)+" AND role_id = "+migrationPlaceholder(cfg, 2),
		fixtures.userTwo,
		fixtures.roleOne,
	).Scan(&storedCreatedAt))
	require.WithinDuration(t, recreatedAt, storedCreatedAt, time.Microsecond)

	_, err = db.ExecContext(ctx, deleteSQL, fixtures.userTwo, fixtures.roleOne)
	require.NoError(t, err)
	_, err = db.ExecContext(
		ctx,
		"DELETE FROM roles WHERE id = "+migrationPlaceholder(cfg, 1),
		fixtures.roleOne,
	)
	require.NoError(t, err, "explicit detach must release role deletion")
	require.Equal(t, 1, m13AssignmentCount(t, ctx, db), "the independent control assignment must survive")
	m13RequireAssignmentCount(t, ctx, db, cfg, fixtures.userControl, fixtures.roleOther, 1)
}

func m13RequireAssignmentCount(
	t *testing.T,
	ctx context.Context,
	db *sql.DB,
	cfg config.DatabaseConfig,
	userID any,
	roleID any,
	expected int,
) {
	t.Helper()

	var count int
	require.NoError(t, db.QueryRowContext(
		ctx,
		"SELECT COUNT(*) FROM system_role_assignments WHERE user_id = "+
			migrationPlaceholder(cfg, 1)+" AND role_id = "+migrationPlaceholder(cfg, 2),
		userID,
		roleID,
	).Scan(&count))
	require.Equal(t, expected, count)
}

func m13RequireParentExists(
	t *testing.T,
	ctx context.Context,
	db *sql.DB,
	cfg config.DatabaseConfig,
	table string,
	id any,
) {
	t.Helper()

	var count int
	require.NoError(t, db.QueryRowContext(
		ctx,
		"SELECT COUNT(*) FROM "+table+" WHERE id = "+migrationPlaceholder(cfg, 1),
		id,
	).Scan(&count))
	require.Equal(t, 1, count)
}
