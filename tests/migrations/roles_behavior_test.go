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

func m10AssertRolesBehavior(
	t *testing.T,
	ctx context.Context,
	db *sql.DB,
	cfg config.DatabaseConfig,
	seedOffset byte,
) {
	t.Helper()

	accountOne := m1AccountID(t, cfg, 1+seedOffset)
	accountTwo := m1AccountID(t, cfg, 2+seedOffset)
	unusedAccount := m1AccountID(t, cfg, 3+seedOffset)
	m2InsertAccount(t, ctx, db, cfg, accountOne, fmt.Sprintf("role-account-one-%d", seedOffset))
	m2InsertAccount(t, ctx, db, cfg, accountTwo, fmt.Sprintf("role-account-two-%d", seedOffset))
	m2InsertAccount(t, ctx, db, cfg, unusedAccount, fmt.Sprintf("role-unused-account-%d", seedOffset))

	createdAt := time.Now().UTC().Truncate(time.Microsecond)
	updatedAt := createdAt.Add(time.Second)
	insertSQL := m10RoleInsertSQL(cfg)
	sharedName := "Administrator"

	systemRoleID := m10RoleID(t, cfg, 10+seedOffset)
	_, err := db.ExecContext(
		ctx,
		insertSQL,
		systemRoleID,
		"system",
		nil,
		sharedName,
		1,
		createdAt,
		updatedAt,
	)
	require.NoError(t, err)

	accountRoleID := m10RoleID(t, cfg, 11+seedOffset)
	_, err = db.ExecContext(
		ctx,
		insertSQL,
		accountRoleID,
		"account",
		accountOne,
		sharedName,
		1,
		createdAt,
		updatedAt,
	)
	require.NoError(t, err, "a display label is not a global role identity")

	_, err = db.ExecContext(
		ctx,
		insertSQL,
		m10RoleID(t, cfg, 12+seedOffset),
		"account",
		accountTwo,
		sharedName,
		1,
		createdAt,
		updatedAt,
	)
	require.NoError(t, err, "different accounts may reuse a display label")

	m10AssertDuplicateLabelsAreAuthorityNeutral(
		t, ctx, db, cfg, insertSQL, accountOne, sharedName, createdAt, updatedAt, seedOffset,
	)
	m10AssertRoleDefaults(t, ctx, db, cfg, accountOne, seedOffset)
	m10AssertRoleOwnership(t, ctx, db, cfg, insertSQL, accountOne, createdAt, updatedAt, seedOffset)
	m10AssertRoleConstraints(t, ctx, db, cfg, insertSQL, accountOne, createdAt, updatedAt, seedOffset)
	m10AssertRoleLifecycle(t, ctx, db, cfg, accountRoleID, updatedAt)

	_, err = db.ExecContext(
		ctx,
		"DELETE FROM accounts WHERE id = "+migrationPlaceholder(cfg, 1),
		accountOne,
	)
	require.Error(t, err, "an account referenced by roles must not be deleted")
	_, err = db.ExecContext(
		ctx,
		"DELETE FROM accounts WHERE id = "+migrationPlaceholder(cfg, 1),
		unusedAccount,
	)
	require.NoError(t, err, "the FK control account without roles must remain deletable")
}

func m10AssertDuplicateLabelsAreAuthorityNeutral(
	t *testing.T,
	ctx context.Context,
	db *sql.DB,
	cfg config.DatabaseConfig,
	insertSQL string,
	accountID any,
	name string,
	createdAt time.Time,
	updatedAt time.Time,
	seedOffset byte,
) {
	t.Helper()

	_, err := db.ExecContext(
		ctx,
		insertSQL,
		m10RoleID(t, cfg, 13+seedOffset),
		"account",
		accountID,
		name,
		1,
		createdAt,
		updatedAt,
	)
	require.NoError(t, err, "duplicate labels in one account must not become identity")

	_, err = db.ExecContext(
		ctx,
		insertSQL,
		m10RoleID(t, cfg, 14+seedOffset),
		"system",
		nil,
		name,
		1,
		createdAt,
		updatedAt,
	)
	require.NoError(t, err, "duplicate system labels must not become identity")

	_, err = db.ExecContext(
		ctx,
		insertSQL,
		m10RoleID(t, cfg, 15+seedOffset),
		"account",
		accountID,
		"admin",
		1,
		createdAt,
		updatedAt,
	)
	require.NoError(t, err)
	_, err = db.ExecContext(
		ctx,
		insertSQL,
		m10RoleID(t, cfg, 16+seedOffset),
		"account",
		accountID,
		"Admin",
		1,
		createdAt,
		updatedAt,
	)
	require.NoError(t, err, "display-label case must not imply role authority")
}

func m10AssertRoleDefaults(
	t *testing.T,
	ctx context.Context,
	db *sql.DB,
	cfg config.DatabaseConfig,
	accountID any,
	seedOffset byte,
) {
	t.Helper()

	roleID := m10RoleID(t, cfg, 17+seedOffset)
	_, err := db.ExecContext(
		ctx,
		"INSERT INTO roles (id, owner_type, account_id, name) VALUES ("+
			migrationPlaceholders(cfg, 4)+")",
		roleID,
		"account",
		accountID,
		"Default version role",
	)
	require.NoError(t, err)

	var version int64
	var createdAt time.Time
	var updatedAt time.Time
	require.NoError(t, db.QueryRowContext(
		ctx,
		"SELECT version, created_at, updated_at FROM roles WHERE id = "+
			migrationPlaceholder(cfg, 1),
		roleID,
	).Scan(&version, &createdAt, &updatedAt))
	require.Equal(t, int64(1), version)
	require.False(t, createdAt.IsZero())
	require.False(t, updatedAt.IsZero())
	require.False(t, updatedAt.Before(createdAt))
}

func m10AssertRoleLifecycle(
	t *testing.T,
	ctx context.Context,
	db *sql.DB,
	cfg config.DatabaseConfig,
	roleID any,
	updatedAt time.Time,
) {
	t.Helper()

	nextUpdatedAt := updatedAt.Add(time.Second)
	result, err := db.ExecContext(
		ctx,
		"UPDATE roles SET name = "+migrationPlaceholder(cfg, 1)+
			", version = version + 1, updated_at = "+migrationPlaceholder(cfg, 2)+
			" WHERE id = "+migrationPlaceholder(cfg, 3)+
			" AND version = "+migrationPlaceholder(cfg, 4),
		"Renamed role",
		nextUpdatedAt,
		roleID,
		1,
	)
	require.NoError(t, err)
	affected, err := result.RowsAffected()
	require.NoError(t, err)
	require.Equal(t, int64(1), affected)

	result, err = db.ExecContext(
		ctx,
		"UPDATE roles SET name = "+migrationPlaceholder(cfg, 1)+
			" WHERE id = "+migrationPlaceholder(cfg, 2)+
			" AND version = "+migrationPlaceholder(cfg, 3),
		"Stale write",
		roleID,
		1,
	)
	require.NoError(t, err)
	affected, err = result.RowsAffected()
	require.NoError(t, err)
	require.Zero(t, affected, "stale optimistic writes must not change the role")

	var name string
	var version int64
	var storedUpdatedAt time.Time
	require.NoError(t, db.QueryRowContext(
		ctx,
		"SELECT name, version, updated_at FROM roles WHERE id = "+
			migrationPlaceholder(cfg, 1),
		roleID,
	).Scan(&name, &version, &storedUpdatedAt))
	require.Equal(t, "Renamed role", name)
	require.Equal(t, int64(2), version)
	require.WithinDuration(t, nextUpdatedAt, storedUpdatedAt, time.Microsecond)
}

func m10RoleInsertSQL(cfg config.DatabaseConfig) string {
	return "INSERT INTO roles (id, owner_type, account_id, name, version, created_at, updated_at) " +
		"VALUES (" + migrationPlaceholders(cfg, 7) + ")"
}

func m10RoleID(t *testing.T, cfg config.DatabaseConfig, seed byte) any {
	t.Helper()

	if cfg.Driver == "postgres" {
		return fmt.Sprintf("%08x-0000-4000-8000-%012x", seed, seed)
	}
	if cfg.Driver == "mysql" {
		return bytes.Repeat([]byte{seed}, 16)
	}
	t.Fatalf("unsupported M10 database profile")
	return nil
}
