package migrations_test

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/korwlik23/go-api-starter/internal/platform/config"
	"github.com/stretchr/testify/require"
)

func m10AssertRoleOwnership(
	t *testing.T,
	ctx context.Context,
	db *sql.DB,
	cfg config.DatabaseConfig,
	insertSQL string,
	accountID any,
	createdAt time.Time,
	updatedAt time.Time,
	seedOffset byte,
) {
	t.Helper()

	unknownAccount := m1AccountID(t, cfg, 90+seedOffset)
	tests := []struct {
		name      string
		ownerType string
		accountID any
	}{
		{name: "system role with account", ownerType: "system", accountID: accountID},
		{name: "account role without account", ownerType: "account"},
		{name: "unknown owner type", ownerType: "tenant", accountID: accountID},
		{name: "uppercase owner type", ownerType: "ACCOUNT", accountID: accountID},
		{name: "padded owner type", ownerType: "account ", accountID: accountID},
		{name: "unknown account", ownerType: "account", accountID: unknownAccount},
	}
	for index, test := range tests {
		_, err := db.ExecContext(
			ctx,
			insertSQL,
			m10RoleID(t, cfg, byte(20+index)+seedOffset),
			test.ownerType,
			test.accountID,
			fmt.Sprintf("Invalid ownership %d", index),
			1,
			createdAt,
			updatedAt,
		)
		require.Error(t, err, test.name)
	}

	m10AssertRequiredRoleIdentity(t, ctx, db, cfg, accountID, seedOffset)
}

func m10AssertRequiredRoleIdentity(
	t *testing.T,
	ctx context.Context,
	db *sql.DB,
	cfg config.DatabaseConfig,
	accountID any,
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
			statement: "INSERT INTO roles (owner_type, account_id, name) VALUES (" +
				migrationPlaceholders(cfg, 3) + ")",
			arguments: []any{"account", accountID, "Missing ID"},
		},
		{
			name: "owner_type",
			statement: "INSERT INTO roles (id, account_id, name) VALUES (" +
				migrationPlaceholders(cfg, 3) + ")",
			arguments: []any{m10RoleID(t, cfg, 30+seedOffset), accountID, "Missing owner type"},
		},
		{
			name: "name",
			statement: "INSERT INTO roles (id, owner_type, account_id) VALUES (" +
				migrationPlaceholders(cfg, 3) + ")",
			arguments: []any{m10RoleID(t, cfg, 31+seedOffset), "account", accountID},
		},
	}
	for _, test := range tests {
		_, err := db.ExecContext(ctx, test.statement, test.arguments...)
		require.Error(t, err, "%s must be explicit", test.name)
	}
}

func m10AssertRoleConstraints(
	t *testing.T,
	ctx context.Context,
	db *sql.DB,
	cfg config.DatabaseConfig,
	insertSQL string,
	accountID any,
	createdAt time.Time,
	updatedAt time.Time,
	seedOffset byte,
) {
	t.Helper()

	m10AssertRoleNameConstraints(t, ctx, db, cfg, insertSQL, accountID, createdAt, updatedAt, seedOffset)
	m10AssertMySQLNonStrictRoleNameBoundary(t, ctx, db, cfg, insertSQL, accountID, createdAt, updatedAt, seedOffset)
	m10AssertRoleStateConstraints(t, ctx, db, cfg, insertSQL, accountID, createdAt, updatedAt, seedOffset)
}

func m10AssertRoleNameConstraints(
	t *testing.T,
	ctx context.Context,
	db *sql.DB,
	cfg config.DatabaseConfig,
	insertSQL string,
	accountID any,
	createdAt time.Time,
	updatedAt time.Time,
	seedOffset byte,
) {
	t.Helper()

	validNames := []string{
		"ก",
		"ผู้ดูแลระบบ",
		"Content editor",
		strings.Repeat("น", 128),
	}
	for index, name := range validNames {
		_, err := db.ExecContext(
			ctx,
			insertSQL,
			m10RoleID(t, cfg, byte(32+index)+seedOffset),
			"account",
			accountID,
			name,
			1,
			createdAt,
			updatedAt,
		)
		require.NoError(t, err, "valid Unicode display label %d must be accepted", index)
	}

	invalidNames := []string{"", " Leading space", "Trailing space ", strings.Repeat("n", 129)}
	for index, name := range invalidNames {
		_, err := db.ExecContext(
			ctx,
			insertSQL,
			m10RoleID(t, cfg, byte(36+index)+seedOffset),
			"account",
			accountID,
			name,
			1,
			createdAt,
			updatedAt,
		)
		require.Error(t, err, "invalid display label %d must be rejected", index)
	}
}

func m10AssertMySQLNonStrictRoleNameBoundary(
	t *testing.T,
	ctx context.Context,
	db *sql.DB,
	cfg config.DatabaseConfig,
	insertSQL string,
	accountID any,
	createdAt time.Time,
	updatedAt time.Time,
	seedOffset byte,
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

	roleID := m10RoleID(t, cfg, 40+seedOffset)
	_, err = connection.ExecContext(
		ctx,
		insertSQL,
		roleID,
		"account",
		accountID,
		strings.Repeat("n", 129),
		1,
		createdAt,
		updatedAt,
	)
	require.Error(t, err, "non-strict mode must not truncate an overlong display label")

	var inserted int
	require.NoError(t, connection.QueryRowContext(
		ctx,
		"SELECT COUNT(*) FROM roles WHERE id = "+migrationPlaceholder(cfg, 1),
		roleID,
	).Scan(&inserted))
	require.Zero(t, inserted, "a rejected non-strict insert must leave no truncated row")
}

func m10AssertRoleStateConstraints(
	t *testing.T,
	ctx context.Context,
	db *sql.DB,
	cfg config.DatabaseConfig,
	insertSQL string,
	accountID any,
	createdAt time.Time,
	updatedAt time.Time,
	seedOffset byte,
) {
	t.Helper()

	tests := []struct {
		name      string
		version   int64
		updatedAt time.Time
	}{
		{name: "zero version", version: 0, updatedAt: updatedAt},
		{name: "negative version", version: -1, updatedAt: updatedAt},
		{name: "updated before created", version: 1, updatedAt: createdAt.Add(-time.Second)},
	}
	for index, test := range tests {
		_, err := db.ExecContext(
			ctx,
			insertSQL,
			m10RoleID(t, cfg, byte(41+index)+seedOffset),
			"account",
			accountID,
			fmt.Sprintf("Invalid state %d", index),
			test.version,
			createdAt,
			test.updatedAt,
		)
		require.Error(t, err, test.name)
	}

	_, err := db.ExecContext(
		ctx,
		insertSQL,
		m10RoleID(t, cfg, 44+seedOffset),
		"account",
		accountID,
		"Duplicate ID control",
		1,
		createdAt,
		updatedAt,
	)
	require.NoError(t, err)
	_, err = db.ExecContext(
		ctx,
		insertSQL,
		m10RoleID(t, cfg, 44+seedOffset),
		"system",
		nil,
		"Different valid label",
		1,
		createdAt,
		updatedAt,
	)
	require.Error(t, err, "role IDs must be globally unique")
}
