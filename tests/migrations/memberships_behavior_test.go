package migrations_test

import (
	"bytes"
	"context"
	"database/sql"
	"fmt"
	"testing"
	"time"

	"github.com/korwlik23/go-api-starter/internal/platform/config"
	"github.com/stretchr/testify/require"
)

func m2AssertMembershipBehavior(
	t *testing.T,
	ctx context.Context,
	db *sql.DB,
	cfg config.DatabaseConfig,
	seedOffset byte,
) {
	t.Helper()

	accountOne := m1AccountID(t, cfg, 10+seedOffset)
	accountTwo := m1AccountID(t, cfg, 11+seedOffset)
	userOne := m3UserID(t, cfg, 20+seedOffset)
	userTwo := m3UserID(t, cfg, 21+seedOffset)
	m2InsertAccount(
		t,
		ctx,
		db,
		cfg,
		accountOne,
		fmt.Sprintf("membership-account-one-%d", seedOffset),
	)
	m2InsertAccount(
		t,
		ctx,
		db,
		cfg,
		accountTwo,
		fmt.Sprintf("membership-account-two-%d", seedOffset),
	)
	m2InsertUser(
		t,
		ctx,
		db,
		cfg,
		userOne,
		fmt.Sprintf("membership-one-%d@example.test", seedOffset),
	)
	m2InsertUser(
		t,
		ctx,
		db,
		cfg,
		userTwo,
		fmt.Sprintf("membership-two-%d@example.test", seedOffset),
	)

	firstMembership := m2MembershipID(t, cfg, 30+seedOffset)
	insertSQL := "INSERT INTO memberships (id, account_id, user_id, status) VALUES (" +
		migrationPlaceholders(cfg, 4) + ")"
	_, err := db.ExecContext(
		ctx,
		insertSQL,
		firstMembership,
		accountOne,
		userOne,
		"custom-status",
	)
	require.NoError(t, err)
	_, err = db.ExecContext(
		ctx,
		insertSQL,
		m2MembershipID(t, cfg, 31+seedOffset),
		accountOne,
		userOne,
		"different-status",
	)
	require.Error(t, err, "duplicate account and user pair must be rejected")
	_, err = db.ExecContext(
		ctx,
		insertSQL,
		m2MembershipID(t, cfg, 32+seedOffset),
		accountTwo,
		userOne,
		"custom-status",
	)
	require.NoError(t, err, "one user may belong to multiple accounts")
	_, err = db.ExecContext(
		ctx,
		insertSQL,
		m2MembershipID(t, cfg, 33+seedOffset),
		accountOne,
		userTwo,
		"custom-status",
	)
	require.NoError(t, err, "one account may contain multiple users")
	_, err = db.ExecContext(
		ctx,
		insertSQL,
		firstMembership,
		accountTwo,
		userTwo,
		"custom-status",
	)
	require.Error(t, err, "duplicate membership ID must be rejected")
	_, err = db.ExecContext(
		ctx,
		insertSQL,
		m2MembershipID(t, cfg, 34+seedOffset),
		m1AccountID(t, cfg, 99+seedOffset),
		userTwo,
		"custom-status",
	)
	require.Error(t, err, "unknown account must be rejected")
	_, err = db.ExecContext(
		ctx,
		insertSQL,
		m2MembershipID(t, cfg, 35+seedOffset),
		accountTwo,
		m3UserID(t, cfg, 99+seedOffset),
		"custom-status",
	)
	require.Error(t, err, "unknown user must be rejected")
	m2AssertRequiredValues(t, ctx, db, cfg, accountTwo, userTwo, seedOffset)

	_, err = db.ExecContext(
		ctx,
		"INSERT INTO memberships "+
			"(id, account_id, user_id, status, authorization_version) VALUES ("+
			migrationPlaceholders(cfg, 5)+")",
		m2MembershipID(t, cfg, 43+seedOffset),
		accountTwo,
		userTwo,
		"custom-status",
		0,
	)
	require.Error(t, err, "authorization version must remain positive")

	var authorizationVersion int64
	var createdAt time.Time
	var updatedAt time.Time
	err = db.QueryRowContext(
		ctx,
		"SELECT authorization_version, created_at, updated_at "+
			"FROM memberships WHERE id = "+migrationPlaceholder(cfg, 1),
		firstMembership,
	).Scan(&authorizationVersion, &createdAt, &updatedAt)
	require.NoError(t, err)
	require.Equal(t, int64(1), authorizationVersion)
	require.False(t, createdAt.IsZero())
	require.False(t, updatedAt.IsZero())

	_, err = db.ExecContext(
		ctx,
		"UPDATE memberships SET authorization_version = authorization_version + 1 "+
			"WHERE id = "+migrationPlaceholder(cfg, 1),
		firstMembership,
	)
	require.NoError(t, err)
	require.NoError(t, db.QueryRowContext(
		ctx,
		"SELECT authorization_version FROM memberships WHERE id = "+
			migrationPlaceholder(cfg, 1),
		firstMembership,
	).Scan(&authorizationVersion))
	require.Equal(t, int64(2), authorizationVersion)

	_, err = db.ExecContext(
		ctx,
		"DELETE FROM accounts WHERE id = "+migrationPlaceholder(cfg, 1),
		accountOne,
	)
	require.Error(t, err, "referenced account deletion must be restricted")
	_, err = db.ExecContext(
		ctx,
		"DELETE FROM users WHERE id = "+migrationPlaceholder(cfg, 1),
		userOne,
	)
	require.Error(t, err, "referenced user deletion must be restricted")
}

func m2AssertRequiredValues(
	t *testing.T,
	ctx context.Context,
	db *sql.DB,
	cfg config.DatabaseConfig,
	accountID any,
	userID any,
	seedOffset byte,
) {
	t.Helper()

	tests := []struct {
		name      string
		statement string
		arguments []any
	}{
		{
			name: "id",
			statement: "INSERT INTO memberships (account_id, user_id, status) VALUES (" +
				migrationPlaceholders(cfg, 3) + ")",
			arguments: []any{accountID, userID, "custom-status"},
		},
		{
			name: "account_id",
			statement: "INSERT INTO memberships (id, user_id, status) VALUES (" +
				migrationPlaceholders(cfg, 3) + ")",
			arguments: []any{
				m2MembershipID(t, cfg, 40+seedOffset),
				userID,
				"custom-status",
			},
		},
		{
			name: "user_id",
			statement: "INSERT INTO memberships (id, account_id, status) VALUES (" +
				migrationPlaceholders(cfg, 3) + ")",
			arguments: []any{
				m2MembershipID(t, cfg, 41+seedOffset),
				accountID,
				"custom-status",
			},
		},
		{
			name: "status",
			statement: "INSERT INTO memberships (id, account_id, user_id) VALUES (" +
				migrationPlaceholders(cfg, 3) + ")",
			arguments: []any{
				m2MembershipID(t, cfg, 42+seedOffset),
				accountID,
				userID,
			},
		},
	}
	for _, test := range tests {
		_, err := db.ExecContext(ctx, test.statement, test.arguments...)
		require.Error(t, err, "%s must be explicit", test.name)
	}
}

func m2InsertAccount(
	t *testing.T,
	ctx context.Context,
	db *sql.DB,
	cfg config.DatabaseConfig,
	id any,
	slug string,
) {
	t.Helper()

	_, err := db.ExecContext(
		ctx,
		"INSERT INTO accounts (id, slug, status) VALUES ("+
			migrationPlaceholders(cfg, 3)+")",
		id,
		slug,
		"test-status",
	)
	require.NoError(t, err)
}

func m2InsertUser(
	t *testing.T,
	ctx context.Context,
	db *sql.DB,
	cfg config.DatabaseConfig,
	id any,
	email string,
) {
	t.Helper()

	_, err := db.ExecContext(
		ctx,
		"INSERT INTO users (id, normalized_email, status) VALUES ("+
			migrationPlaceholders(cfg, 3)+")",
		id,
		email,
		"test-status",
	)
	require.NoError(t, err)
}

func m2MembershipID(t *testing.T, cfg config.DatabaseConfig, seed byte) any {
	t.Helper()

	if cfg.Driver == "postgres" {
		return fmt.Sprintf("%08x-0000-4000-8000-%012x", seed, seed)
	}
	if cfg.Driver == "mysql" {
		return bytes.Repeat([]byte{seed}, 16)
	}
	t.Fatalf("unsupported M2 database profile")
	return nil
}
