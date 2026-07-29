package migrations_test

import (
	"context"
	"database/sql"
	"fmt"
	"testing"
	"time"

	"github.com/korwlik23/go-api-starter/internal/platform/config"
	"github.com/stretchr/testify/require"
)

type m13Fixtures struct {
	accountOne  any
	userOne     any
	userTwo     any
	userControl any
	roleOne     any
	roleTwo     any
	roleOther   any
	roleControl any
	roleAccount any
}

func m13AssertSystemRoleAssignmentBehavior(
	t *testing.T,
	ctx context.Context,
	db *sql.DB,
	cfg config.DatabaseConfig,
	seedOffset byte,
) {
	t.Helper()

	fixtures := m13CreateFixtures(t, ctx, db, cfg, seedOffset)
	insertSQL := m13SystemRoleAssignmentInsertSQL(cfg)
	for _, assignment := range [][2]any{
		{fixtures.userOne, fixtures.roleOne},
		{fixtures.userOne, fixtures.roleTwo},
		{fixtures.userTwo, fixtures.roleOne},
		{fixtures.userControl, fixtures.roleOther},
	} {
		_, err := db.ExecContext(ctx, insertSQL, assignment[0], "system", assignment[1])
		require.NoError(t, err)
	}

	m13AssertMappingIdentity(t, ctx, db, cfg, insertSQL, fixtures, seedOffset)
	m13AssertRequiredAssignmentValues(t, ctx, db, cfg, fixtures)
	m13AssertAssignmentProvenance(t, ctx, db, cfg, fixtures)
	m13AssertRoleOwnerConsistency(t, ctx, db, cfg, insertSQL, fixtures)
	m13AssertParentUpdatesRestricted(t, ctx, db, cfg, fixtures, seedOffset)
	m13AssertDeletePolicies(t, ctx, db, cfg, fixtures)
}

func m13CreateFixtures(
	t *testing.T,
	ctx context.Context,
	db *sql.DB,
	cfg config.DatabaseConfig,
	seedOffset byte,
) m13Fixtures {
	t.Helper()

	fixtures := m13Fixtures{
		accountOne:  m1AccountID(t, cfg, 1+seedOffset),
		userOne:     m3UserID(t, cfg, 5+seedOffset),
		userTwo:     m3UserID(t, cfg, 6+seedOffset),
		userControl: m3UserID(t, cfg, 7+seedOffset),
		roleOne:     m10RoleID(t, cfg, 10+seedOffset),
		roleTwo:     m10RoleID(t, cfg, 11+seedOffset),
		roleOther:   m10RoleID(t, cfg, 12+seedOffset),
		roleControl: m10RoleID(t, cfg, 13+seedOffset),
		roleAccount: m10RoleID(t, cfg, 14+seedOffset),
	}
	m2InsertAccount(t, ctx, db, cfg, fixtures.accountOne, fmt.Sprintf("system-assignment-%d", seedOffset))
	for index, userID := range []any{fixtures.userOne, fixtures.userTwo, fixtures.userControl} {
		m2InsertUser(
			t,
			ctx,
			db,
			cfg,
			userID,
			fmt.Sprintf("system-assignment-%d-%d@example.test", seedOffset, index),
		)
	}
	m13InsertSystemRole(t, ctx, db, cfg, fixtures.roleOne, "System role one")
	m13InsertSystemRole(t, ctx, db, cfg, fixtures.roleTwo, "System role two")
	m13InsertSystemRole(t, ctx, db, cfg, fixtures.roleOther, "Surviving system role")
	m13InsertSystemRole(t, ctx, db, cfg, fixtures.roleControl, "Unassigned control role")
	m11InsertAccountRole(t, ctx, db, cfg, fixtures.roleAccount, fixtures.accountOne, "Account-only role")

	return fixtures
}

func m13AssertMappingIdentity(
	t *testing.T,
	ctx context.Context,
	db *sql.DB,
	cfg config.DatabaseConfig,
	insertSQL string,
	fixtures m13Fixtures,
	seedOffset byte,
) {
	t.Helper()

	_, err := db.ExecContext(ctx, insertSQL, fixtures.userOne, "system", fixtures.roleOne)
	require.Error(t, err, "an exact user-role assignment must be unique")
	_, err = db.ExecContext(
		ctx,
		insertSQL,
		m3UserID(t, cfg, 90+seedOffset),
		"system",
		fixtures.roleOne,
	)
	require.Error(t, err, "an unknown user must be rejected")
	_, err = db.ExecContext(
		ctx,
		insertSQL,
		fixtures.userOne,
		"system",
		m10RoleID(t, cfg, 90+seedOffset),
	)
	require.Error(t, err, "an unknown role must be rejected")
	require.Equal(t, 4, m13AssignmentCount(t, ctx, db), "failed inserts must not alter assignments")
}

func m13AssertRequiredAssignmentValues(
	t *testing.T,
	ctx context.Context,
	db *sql.DB,
	cfg config.DatabaseConfig,
	fixtures m13Fixtures,
) {
	t.Helper()

	tests := []struct {
		name      string
		statement string
		arguments []any
	}{
		{
			name: "user_id",
			statement: "INSERT INTO system_role_assignments (role_owner_type, role_id) VALUES (" +
				migrationPlaceholders(cfg, 2) + ")",
			arguments: []any{"system", fixtures.roleOne},
		},
		{
			name: "role_owner_type",
			statement: "INSERT INTO system_role_assignments (user_id, role_id) VALUES (" +
				migrationPlaceholders(cfg, 2) + ")",
			arguments: []any{fixtures.userOne, fixtures.roleOne},
		},
		{
			name: "role_id",
			statement: "INSERT INTO system_role_assignments (user_id, role_owner_type) VALUES (" +
				migrationPlaceholders(cfg, 2) + ")",
			arguments: []any{fixtures.userOne, "system"},
		},
	}
	for _, test := range tests {
		_, err := db.ExecContext(ctx, test.statement, test.arguments...)
		require.Error(t, err, "%s must be explicit", test.name)
	}
	require.Equal(t, 4, m13AssignmentCount(t, ctx, db))
}

func m13AssertAssignmentProvenance(
	t *testing.T,
	ctx context.Context,
	db *sql.DB,
	cfg config.DatabaseConfig,
	fixtures m13Fixtures,
) {
	t.Helper()

	var ownerType string
	var createdAt time.Time
	require.NoError(t, db.QueryRowContext(
		ctx,
		"SELECT role_owner_type, created_at FROM system_role_assignments WHERE user_id = "+
			migrationPlaceholder(cfg, 1)+" AND role_id = "+migrationPlaceholder(cfg, 2),
		fixtures.userOne,
		fixtures.roleOne,
	).Scan(&ownerType, &createdAt))
	require.Equal(t, "system", ownerType)
	require.False(t, createdAt.IsZero())
}

func m13SystemRoleAssignmentInsertSQL(cfg config.DatabaseConfig) string {
	return "INSERT INTO system_role_assignments (user_id, role_owner_type, role_id) VALUES (" +
		migrationPlaceholders(cfg, 3) + ")"
}

func m13InsertSystemRole(
	t *testing.T,
	ctx context.Context,
	db *sql.DB,
	cfg config.DatabaseConfig,
	roleID any,
	name string,
) {
	t.Helper()

	_, err := db.ExecContext(
		ctx,
		"INSERT INTO roles (id, owner_type, account_id, name) VALUES ("+
			migrationPlaceholders(cfg, 4)+")",
		roleID,
		"system",
		nil,
		name,
	)
	require.NoError(t, err)
}

func m13AssignmentCount(t *testing.T, ctx context.Context, db *sql.DB) int {
	t.Helper()

	var count int
	require.NoError(t, db.QueryRowContext(
		ctx,
		"SELECT COUNT(*) FROM system_role_assignments",
	).Scan(&count))
	return count
}
