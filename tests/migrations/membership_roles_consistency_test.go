package migrations_test

import (
	"context"
	"database/sql"
	"testing"

	"github.com/korwlik23/go-api-starter/internal/platform/config"
	"github.com/stretchr/testify/require"
)

func m12AssertAccountConsistency(
	t *testing.T,
	ctx context.Context,
	db *sql.DB,
	cfg config.DatabaseConfig,
	insertSQL string,
	fixtures m12Fixtures,
) {
	t.Helper()

	tests := []struct {
		name       string
		accountID  any
		membership any
		role       any
	}{
		{
			name:       "account A cannot use an account B role",
			accountID:  fixtures.accountOne,
			membership: fixtures.membershipOne,
			role:       fixtures.roleOther,
		},
		{
			name:       "account A cannot use an account B membership",
			accountID:  fixtures.accountOne,
			membership: fixtures.membershipOther,
			role:       fixtures.roleOne,
		},
		{
			name:       "account B cannot use an account A role",
			accountID:  fixtures.accountTwo,
			membership: fixtures.membershipOther,
			role:       fixtures.roleOne,
		},
		{
			name:       "account B cannot use an account A membership",
			accountID:  fixtures.accountTwo,
			membership: fixtures.membershipOne,
			role:       fixtures.roleOther,
		},
		{
			name:       "an account membership cannot receive a system role",
			accountID:  fixtures.accountOne,
			membership: fixtures.membershipOne,
			role:       fixtures.systemRole,
		},
	}
	for _, test := range tests {
		_, err := db.ExecContext(
			ctx,
			insertSQL,
			test.accountID,
			test.membership,
			test.role,
		)
		require.Error(t, err, test.name)
	}
	require.Equal(t, 4, m12AssignmentCount(t, ctx, db), "rejected cross-account rows must not persist")
}

func m12AssertParentUpdatesRestricted(
	t *testing.T,
	ctx context.Context,
	db *sql.DB,
	cfg config.DatabaseConfig,
	fixtures m12Fixtures,
	seedOffset byte,
) {
	t.Helper()

	tests := []struct {
		name      string
		statement string
		arguments []any
	}{
		{
			name: "mapped role account",
			statement: "UPDATE roles SET account_id = " + migrationPlaceholder(cfg, 1) +
				" WHERE id = " + migrationPlaceholder(cfg, 2),
			arguments: []any{fixtures.accountTwo, fixtures.roleTwo},
		},
		{
			name: "mapped account role owner transition",
			statement: "UPDATE roles SET owner_type = " + migrationPlaceholder(cfg, 1) +
				", account_id = " + migrationPlaceholder(cfg, 2) +
				" WHERE id = " + migrationPlaceholder(cfg, 3),
			arguments: []any{"system", nil, fixtures.roleTwo},
		},
		{
			name: "mapped membership account",
			statement: "UPDATE memberships SET account_id = " + migrationPlaceholder(cfg, 1) +
				" WHERE id = " + migrationPlaceholder(cfg, 2),
			arguments: []any{fixtures.accountTwo, fixtures.membershipTwo},
		},
		{
			name: "mapped role identity",
			statement: "UPDATE roles SET id = " + migrationPlaceholder(cfg, 1) +
				" WHERE id = " + migrationPlaceholder(cfg, 2),
			arguments: []any{m10RoleID(t, cfg, 15+seedOffset), fixtures.roleOne},
		},
		{
			name: "mapped membership identity",
			statement: "UPDATE memberships SET id = " + migrationPlaceholder(cfg, 1) +
				" WHERE id = " + migrationPlaceholder(cfg, 2),
			arguments: []any{m2MembershipID(t, cfg, 33+seedOffset), fixtures.membershipOne},
		},
	}
	for _, test := range tests {
		_, err := db.ExecContext(ctx, test.statement, test.arguments...)
		require.Error(t, err, "%s updates must be restricted while assigned", test.name)
	}
	require.Equal(t, 4, m12AssignmentCount(t, ctx, db))
}

func m12AssertDeletePolicies(
	t *testing.T,
	ctx context.Context,
	db *sql.DB,
	cfg config.DatabaseConfig,
	insertSQL string,
	fixtures m12Fixtures,
) {
	t.Helper()

	_, err := db.ExecContext(
		ctx,
		"DELETE FROM roles WHERE id = "+migrationPlaceholder(cfg, 1),
		fixtures.roleOne,
	)
	require.Error(t, err, "assigned roles require an explicit audited detach")
	_, err = db.ExecContext(
		ctx,
		"DELETE FROM roles WHERE id = "+migrationPlaceholder(cfg, 1),
		fixtures.roleControl,
	)
	require.NoError(t, err, "an unassigned role must remain deletable")
	require.Equal(t, 4, m12AssignmentCount(t, ctx, db))

	_, err = db.ExecContext(
		ctx,
		"DELETE FROM memberships WHERE id = "+migrationPlaceholder(cfg, 1),
		fixtures.membershipOne,
	)
	require.NoError(t, err, "membership deletion must cascade its assignments")
	require.Equal(t, 2, m12AssignmentCount(t, ctx, db))
	m12RequireParentExists(t, ctx, db, cfg, m10RolesTable, fixtures.roleOne)
	m12RequireParentExists(t, ctx, db, cfg, m10RolesTable, fixtures.roleTwo)

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
	require.Error(t, err, "another membership still holds this role")

	deleteSQL := "DELETE FROM membership_roles WHERE account_id = " +
		migrationPlaceholder(cfg, 1) + " AND membership_id = " +
		migrationPlaceholder(cfg, 2) + " AND role_id = " + migrationPlaceholder(cfg, 3)
	_, err = db.ExecContext(
		ctx,
		deleteSQL,
		fixtures.accountOne,
		fixtures.membershipTwo,
		fixtures.roleOne,
	)
	require.NoError(t, err)
	m12RequireParentExists(t, ctx, db, cfg, m2MembershipsTable, fixtures.membershipTwo)
	m12RequireParentExists(t, ctx, db, cfg, m10RolesTable, fixtures.roleOne)

	_, err = db.ExecContext(
		ctx,
		insertSQL,
		fixtures.accountOne,
		fixtures.membershipTwo,
		fixtures.roleOne,
	)
	require.NoError(t, err)
	_, err = db.ExecContext(
		ctx,
		deleteSQL,
		fixtures.accountOne,
		fixtures.membershipTwo,
		fixtures.roleOne,
	)
	require.NoError(t, err)
	_, err = db.ExecContext(
		ctx,
		"DELETE FROM roles WHERE id = "+migrationPlaceholder(cfg, 1),
		fixtures.roleOne,
	)
	require.NoError(t, err, "explicit detach must release the role deletion")

	require.Equal(t, 1, m12AssignmentCount(t, ctx, db), "the other account assignment must remain")
	m12RequireParentExists(t, ctx, db, cfg, m2MembershipsTable, fixtures.membershipOther)
	m12RequireParentExists(t, ctx, db, cfg, m10RolesTable, fixtures.roleOther)
}

func m12RequireParentExists(
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
