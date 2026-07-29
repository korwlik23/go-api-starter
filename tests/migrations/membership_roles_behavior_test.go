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

type m12Fixtures struct {
	accountOne      any
	accountTwo      any
	membershipOne   any
	membershipTwo   any
	membershipOther any
	roleOne         any
	roleTwo         any
	roleOther       any
	systemRole      any
	roleControl     any
}

func m12AssertMembershipRoleBehavior(
	t *testing.T,
	ctx context.Context,
	db *sql.DB,
	cfg config.DatabaseConfig,
	seedOffset byte,
) {
	t.Helper()

	fixtures := m12CreateFixtures(t, ctx, db, cfg, seedOffset)
	insertSQL := m12MembershipRoleInsertSQL(cfg)
	for _, assignment := range [][3]any{
		{fixtures.accountOne, fixtures.membershipOne, fixtures.roleOne},
		{fixtures.accountOne, fixtures.membershipOne, fixtures.roleTwo},
		{fixtures.accountOne, fixtures.membershipTwo, fixtures.roleOne},
		{fixtures.accountTwo, fixtures.membershipOther, fixtures.roleOther},
	} {
		_, err := db.ExecContext(ctx, insertSQL, assignment[:]...)
		require.NoError(t, err)
	}

	m12AssertMappingIdentity(t, ctx, db, cfg, insertSQL, fixtures, seedOffset)
	m12AssertRequiredAssignmentValues(t, ctx, db, cfg, fixtures)
	m12AssertAssignmentTimestamp(t, ctx, db, cfg, fixtures)
	m12AssertAccountConsistency(t, ctx, db, cfg, insertSQL, fixtures)
	m12AssertParentUpdatesRestricted(t, ctx, db, cfg, fixtures, seedOffset)
	m12AssertDeletePolicies(t, ctx, db, cfg, insertSQL, fixtures)
}

func m12CreateFixtures(
	t *testing.T,
	ctx context.Context,
	db *sql.DB,
	cfg config.DatabaseConfig,
	seedOffset byte,
) m12Fixtures {
	t.Helper()

	fixtures := m12Fixtures{
		accountOne:      m1AccountID(t, cfg, 1+seedOffset),
		accountTwo:      m1AccountID(t, cfg, 2+seedOffset),
		membershipOne:   m2MembershipID(t, cfg, 30+seedOffset),
		membershipTwo:   m2MembershipID(t, cfg, 31+seedOffset),
		membershipOther: m2MembershipID(t, cfg, 32+seedOffset),
		roleOne:         m10RoleID(t, cfg, 10+seedOffset),
		roleTwo:         m10RoleID(t, cfg, 11+seedOffset),
		roleOther:       m10RoleID(t, cfg, 12+seedOffset),
		systemRole:      m10RoleID(t, cfg, 13+seedOffset),
		roleControl:     m10RoleID(t, cfg, 14+seedOffset),
	}
	userOne := m3UserID(t, cfg, 5+seedOffset)
	userTwo := m3UserID(t, cfg, 6+seedOffset)
	m2InsertAccount(t, ctx, db, cfg, fixtures.accountOne, fmt.Sprintf("assignment-a-%d", seedOffset))
	m2InsertAccount(t, ctx, db, cfg, fixtures.accountTwo, fmt.Sprintf("assignment-b-%d", seedOffset))
	m2InsertUser(t, ctx, db, cfg, userOne, fmt.Sprintf("assignment-one-%d@example.test", seedOffset))
	m2InsertUser(t, ctx, db, cfg, userTwo, fmt.Sprintf("assignment-two-%d@example.test", seedOffset))

	m12InsertMembership(t, ctx, db, cfg, fixtures.membershipOne, fixtures.accountOne, userOne)
	m12InsertMembership(t, ctx, db, cfg, fixtures.membershipTwo, fixtures.accountOne, userTwo)
	m12InsertMembership(t, ctx, db, cfg, fixtures.membershipOther, fixtures.accountTwo, userOne)
	m11InsertAccountRole(t, ctx, db, cfg, fixtures.roleOne, fixtures.accountOne, "Account A role one")
	m11InsertAccountRole(t, ctx, db, cfg, fixtures.roleTwo, fixtures.accountOne, "Account A role two")
	m11InsertAccountRole(t, ctx, db, cfg, fixtures.roleOther, fixtures.accountTwo, "Account B role")
	m11InsertAccountRole(t, ctx, db, cfg, fixtures.roleControl, fixtures.accountOne, "Unassigned control role")
	_, err := db.ExecContext(
		ctx,
		"INSERT INTO roles (id, owner_type, account_id, name) VALUES ("+
			migrationPlaceholders(cfg, 4)+")",
		fixtures.systemRole,
		"system",
		nil,
		"System role",
	)
	require.NoError(t, err)

	return fixtures
}

func m12AssertMappingIdentity(
	t *testing.T,
	ctx context.Context,
	db *sql.DB,
	cfg config.DatabaseConfig,
	insertSQL string,
	fixtures m12Fixtures,
	seedOffset byte,
) {
	t.Helper()

	_, err := db.ExecContext(
		ctx,
		insertSQL,
		fixtures.accountOne,
		fixtures.membershipOne,
		fixtures.roleOne,
	)
	require.Error(t, err, "an exact membership-role assignment must be unique")
	_, err = db.ExecContext(
		ctx,
		insertSQL,
		fixtures.accountOne,
		m2MembershipID(t, cfg, 90+seedOffset),
		fixtures.roleOne,
	)
	require.Error(t, err, "an unknown membership must be rejected")
	_, err = db.ExecContext(
		ctx,
		insertSQL,
		fixtures.accountOne,
		fixtures.membershipOne,
		m10RoleID(t, cfg, 90+seedOffset),
	)
	require.Error(t, err, "an unknown role must be rejected")

	require.Equal(t, 4, m12AssignmentCount(t, ctx, db), "failed inserts must not alter assignments")
}

func m12AssertRequiredAssignmentValues(
	t *testing.T,
	ctx context.Context,
	db *sql.DB,
	cfg config.DatabaseConfig,
	fixtures m12Fixtures,
) {
	t.Helper()

	tests := []struct {
		name      string
		statement string
		arguments []any
	}{
		{
			name: "account_id",
			statement: "INSERT INTO membership_roles (membership_id, role_id) VALUES (" +
				migrationPlaceholders(cfg, 2) + ")",
			arguments: []any{fixtures.membershipOne, fixtures.roleOne},
		},
		{
			name: "membership_id",
			statement: "INSERT INTO membership_roles (account_id, role_id) VALUES (" +
				migrationPlaceholders(cfg, 2) + ")",
			arguments: []any{fixtures.accountOne, fixtures.roleOne},
		},
		{
			name: "role_id",
			statement: "INSERT INTO membership_roles (account_id, membership_id) VALUES (" +
				migrationPlaceholders(cfg, 2) + ")",
			arguments: []any{fixtures.accountOne, fixtures.membershipOne},
		},
	}
	for _, test := range tests {
		_, err := db.ExecContext(ctx, test.statement, test.arguments...)
		require.Error(t, err, "%s must be explicit", test.name)
	}
	require.Equal(t, 4, m12AssignmentCount(t, ctx, db))
}

func m12AssertAssignmentTimestamp(
	t *testing.T,
	ctx context.Context,
	db *sql.DB,
	cfg config.DatabaseConfig,
	fixtures m12Fixtures,
) {
	t.Helper()

	var createdAt time.Time
	require.NoError(t, db.QueryRowContext(
		ctx,
		"SELECT created_at FROM membership_roles WHERE account_id = "+
			migrationPlaceholder(cfg, 1)+" AND membership_id = "+
			migrationPlaceholder(cfg, 2)+" AND role_id = "+migrationPlaceholder(cfg, 3),
		fixtures.accountOne,
		fixtures.membershipOne,
		fixtures.roleOne,
	).Scan(&createdAt))
	require.False(t, createdAt.IsZero())
}

func m12MembershipRoleInsertSQL(cfg config.DatabaseConfig) string {
	return "INSERT INTO membership_roles (account_id, membership_id, role_id) VALUES (" +
		migrationPlaceholders(cfg, 3) + ")"
}

func m12InsertMembership(
	t *testing.T,
	ctx context.Context,
	db *sql.DB,
	cfg config.DatabaseConfig,
	membershipID any,
	accountID any,
	userID any,
) {
	t.Helper()

	_, err := db.ExecContext(
		ctx,
		"INSERT INTO memberships (id, account_id, user_id, status) VALUES ("+
			migrationPlaceholders(cfg, 4)+")",
		membershipID,
		accountID,
		userID,
		"active",
	)
	require.NoError(t, err)
}

func m12AssignmentCount(t *testing.T, ctx context.Context, db *sql.DB) int {
	t.Helper()

	var count int
	require.NoError(t, db.QueryRowContext(
		ctx,
		"SELECT COUNT(*) FROM membership_roles",
	).Scan(&count))
	return count
}
