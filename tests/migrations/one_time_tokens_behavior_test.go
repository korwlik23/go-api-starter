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

func m8AssertTokenBehavior(
	t *testing.T,
	ctx context.Context,
	db *sql.DB,
	cfg config.DatabaseConfig,
	seedOffset byte,
) {
	t.Helper()

	userOneID := m3UserID(t, cfg, 10+seedOffset)
	userTwoID := m3UserID(t, cfg, 11+seedOffset)
	m2InsertUser(t, ctx, db, cfg, userOneID, fmt.Sprintf("token-one-%d@example.test", seedOffset))
	m2InsertUser(t, ctx, db, cfg, userTwoID, fmt.Sprintf("token-two-%d@example.test", seedOffset))
	createdAt := time.Now().UTC().Truncate(time.Microsecond)
	expiresAt := createdAt.Add(time.Hour)
	insertSQL := "INSERT INTO one_time_tokens (" +
		"id, user_id, purpose, token_hash, hash_version, hash_key_version, " +
		"expires_at, created_at) VALUES (" + migrationPlaceholders(cfg, 4) +
		", 1, 1, " + migrationPlaceholder(cfg, 5) + ", " +
		migrationPlaceholder(cfg, 6) + ")"

	activationHash := m8Hash(20 + seedOffset)
	_, err := db.ExecContext(
		ctx,
		insertSQL,
		m8TokenID(t, cfg, 30+seedOffset),
		userOneID,
		"user_activation",
		activationHash,
		expiresAt,
		createdAt,
	)
	require.NoError(t, err)
	resetHash := m8Hash(21 + seedOffset)
	_, err = db.ExecContext(
		ctx,
		insertSQL,
		m8TokenID(t, cfg, 31+seedOffset),
		userOneID,
		"password_reset",
		resetHash,
		expiresAt,
		createdAt,
	)
	require.NoError(t, err)
	secondResetHash := m8Hash(22 + seedOffset)
	_, err = db.ExecContext(
		ctx,
		insertSQL,
		m8TokenID(t, cfg, 32+seedOffset),
		userOneID,
		"password_reset",
		secondResetHash,
		expiresAt,
		createdAt,
	)
	require.NoError(t, err, "schema must support transactional supersession in the repository")

	m8AssertGlobalHashUniqueness(
		t,
		ctx,
		db,
		cfg,
		insertSQL,
		userOneID,
		userTwoID,
		activationHash,
		createdAt,
		expiresAt,
		seedOffset,
	)
	m8AssertInvalidTokenState(
		t,
		ctx,
		db,
		cfg,
		insertSQL,
		userOneID,
		createdAt,
		expiresAt,
		seedOffset,
	)
	m8AssertTokenConsumption(
		t,
		ctx,
		db,
		cfg,
		insertSQL,
		userOneID,
		createdAt,
		seedOffset,
	)

	var storedHash []byte
	var purpose string
	var consumedAt sql.NullTime
	require.NoError(t, db.QueryRowContext(
		ctx,
		"SELECT purpose, token_hash, consumed_at FROM one_time_tokens WHERE id = "+
			migrationPlaceholder(cfg, 1),
		m8TokenID(t, cfg, 31+seedOffset),
	).Scan(&purpose, &storedHash, &consumedAt))
	require.Equal(t, "password_reset", purpose)
	require.Equal(t, resetHash, storedHash)
	require.False(t, consumedAt.Valid)

	_, err = db.ExecContext(
		ctx,
		"DELETE FROM users WHERE id = "+migrationPlaceholder(cfg, 1),
		userOneID,
	)
	require.NoError(t, err)
	var tokenCount int
	require.NoError(t, db.QueryRowContext(
		ctx,
		"SELECT COUNT(*) FROM one_time_tokens WHERE user_id = "+migrationPlaceholder(cfg, 1),
		userOneID,
	).Scan(&tokenCount))
	require.Zero(t, tokenCount, "one-time tokens must be removed with their user")
}

func m8AssertGlobalHashUniqueness(
	t *testing.T,
	ctx context.Context,
	db *sql.DB,
	cfg config.DatabaseConfig,
	insertSQL string,
	userOneID any,
	userTwoID any,
	tokenHash []byte,
	createdAt time.Time,
	expiresAt time.Time,
	seedOffset byte,
) {
	t.Helper()

	for index, duplicate := range []struct {
		userID  any
		purpose string
	}{
		{userID: userOneID, purpose: "user_activation"},
		{userID: userOneID, purpose: "password_reset"},
		{userID: userTwoID, purpose: "password_reset"},
	} {
		_, err := db.ExecContext(
			ctx,
			insertSQL,
			m8TokenID(t, cfg, byte(40+index)+seedOffset),
			duplicate.userID,
			duplicate.purpose,
			tokenHash,
			expiresAt,
			createdAt,
		)
		require.Error(t, err, "token hash must be globally unique for duplicate %d", index)
	}
}

func m8AssertInvalidTokenState(
	t *testing.T,
	ctx context.Context,
	db *sql.DB,
	cfg config.DatabaseConfig,
	insertSQL string,
	userID any,
	createdAt time.Time,
	expiresAt time.Time,
	seedOffset byte,
) {
	t.Helper()

	for index, purpose := range []string{"activation", "PASSWORD_RESET"} {
		_, err := db.ExecContext(
			ctx,
			insertSQL,
			m8TokenID(t, cfg, byte(50+index)+seedOffset),
			userID,
			purpose,
			m8Hash(byte(50+index)+seedOffset),
			expiresAt,
			createdAt,
		)
		require.Error(t, err, "unknown or case-folded purpose %d must be rejected", index)
	}

	for index, tokenHash := range [][]byte{
		bytes.Repeat([]byte{60 + seedOffset}, 31),
		bytes.Repeat([]byte{61 + seedOffset}, 33),
	} {
		_, err := db.ExecContext(
			ctx,
			insertSQL,
			m8TokenID(t, cfg, byte(60+index)+seedOffset),
			userID,
			"password_reset",
			tokenHash,
			expiresAt,
			createdAt,
		)
		require.Error(t, err, "token hash boundary %d must be rejected", index)
	}

	_, err := db.ExecContext(
		ctx,
		insertSQL,
		m8TokenID(t, cfg, 62+seedOffset),
		m3UserID(t, cfg, 99+seedOffset),
		"password_reset",
		m8Hash(62+seedOffset),
		expiresAt,
		createdAt,
	)
	require.Error(t, err, "a token for an unknown user must be rejected")

	for index, invalidExpiry := range []time.Time{createdAt, createdAt.Add(-time.Second)} {
		_, err := db.ExecContext(
			ctx,
			insertSQL,
			m8TokenID(t, cfg, byte(63+index)+seedOffset),
			userID,
			"password_reset",
			m8Hash(byte(63+index)+seedOffset),
			invalidExpiry,
			createdAt,
		)
		require.Error(t, err, "non-positive token lifetime %d must be rejected", index)
	}

	for index, versions := range [][2]int{{0, 1}, {1, 0}} {
		_, err := db.ExecContext(
			ctx,
			"INSERT INTO one_time_tokens (id, user_id, purpose, token_hash, "+
				"hash_version, hash_key_version, expires_at, created_at) VALUES ("+
				migrationPlaceholders(cfg, 8)+")",
			m8TokenID(t, cfg, byte(66+index)+seedOffset),
			userID,
			"password_reset",
			m8Hash(byte(66+index)+seedOffset),
			versions[0],
			versions[1],
			expiresAt,
			createdAt,
		)
		require.Error(t, err, "non-positive token hash metadata %d must be rejected", index)
	}

	invalidFinalStates := []struct {
		name       string
		consumedAt any
		revokedAt  any
	}{
		{name: "consumed before creation", consumedAt: createdAt.Add(-time.Second)},
		{name: "consumed at expiry", consumedAt: expiresAt},
		{name: "revoked before creation", revokedAt: createdAt.Add(-time.Second)},
		{name: "both consumed and revoked", consumedAt: createdAt.Add(time.Minute), revokedAt: createdAt.Add(2 * time.Minute)},
	}
	for index, state := range invalidFinalStates {
		_, err := db.ExecContext(
			ctx,
			"INSERT INTO one_time_tokens (id, user_id, purpose, token_hash, hash_version, "+
				"hash_key_version, expires_at, consumed_at, revoked_at, created_at) VALUES ("+
				migrationPlaceholders(cfg, 10)+")",
			m8TokenID(t, cfg, byte(70+index)+seedOffset),
			userID,
			"password_reset",
			m8Hash(byte(70+index)+seedOffset),
			1,
			1,
			expiresAt,
			state.consumedAt,
			state.revokedAt,
			createdAt,
		)
		require.Error(t, err, state.name)
	}
}

func m8AssertTokenConsumption(
	t *testing.T,
	ctx context.Context,
	db *sql.DB,
	cfg config.DatabaseConfig,
	insertSQL string,
	userID any,
	createdAt time.Time,
	seedOffset byte,
) {
	t.Helper()

	consumeTime := createdAt.Add(10 * time.Minute)
	consumeSQL := "UPDATE one_time_tokens SET consumed_at = " + migrationPlaceholder(cfg, 1) +
		" WHERE purpose = " + migrationPlaceholder(cfg, 2) +
		" AND token_hash = " + migrationPlaceholder(cfg, 3) +
		" AND consumed_at IS NULL AND revoked_at IS NULL" +
		" AND created_at <= " + migrationPlaceholder(cfg, 4) +
		" AND expires_at > " + migrationPlaceholder(cfg, 5)

	validHash := m8Hash(80 + seedOffset)
	_, err := db.ExecContext(
		ctx,
		insertSQL,
		m8TokenID(t, cfg, 80+seedOffset),
		userID,
		"password_reset",
		validHash,
		createdAt.Add(time.Hour),
		createdAt,
	)
	require.NoError(t, err)
	result, err := db.ExecContext(
		ctx,
		consumeSQL,
		consumeTime,
		"user_activation",
		validHash,
		consumeTime,
		consumeTime,
	)
	require.NoError(t, err)
	affected, err := result.RowsAffected()
	require.NoError(t, err)
	require.Zero(t, affected, "a token must not be consumed for a different purpose")

	result, err = db.ExecContext(
		ctx,
		consumeSQL,
		consumeTime,
		"password_reset",
		validHash,
		consumeTime,
		consumeTime,
	)
	require.NoError(t, err)
	affected, err = result.RowsAffected()
	require.NoError(t, err)
	require.Equal(t, int64(1), affected)
	result, err = db.ExecContext(
		ctx,
		consumeSQL,
		consumeTime.Add(time.Second),
		"password_reset",
		validHash,
		consumeTime.Add(time.Second),
		consumeTime.Add(time.Second),
	)
	require.NoError(t, err)
	affected, err = result.RowsAffected()
	require.NoError(t, err)
	require.Zero(t, affected, "a consumed token must not be accepted again")

	expiredHash := m8Hash(81 + seedOffset)
	_, err = db.ExecContext(
		ctx,
		insertSQL,
		m8TokenID(t, cfg, 81+seedOffset),
		userID,
		"password_reset",
		expiredHash,
		consumeTime,
		createdAt,
	)
	require.NoError(t, err, "expired rows must remain representable for bounded cleanup")
	result, err = db.ExecContext(
		ctx,
		consumeSQL,
		consumeTime,
		"password_reset",
		expiredHash,
		consumeTime,
		consumeTime,
	)
	require.NoError(t, err)
	affected, err = result.RowsAffected()
	require.NoError(t, err)
	require.Zero(t, affected, "a token is expired at the exact expiry boundary")

	revokedHash := m8Hash(82 + seedOffset)
	_, err = db.ExecContext(
		ctx,
		"INSERT INTO one_time_tokens (id, user_id, purpose, token_hash, hash_version, "+
			"hash_key_version, expires_at, revoked_at, created_at) VALUES ("+
			migrationPlaceholders(cfg, 9)+")",
		m8TokenID(t, cfg, 82+seedOffset),
		userID,
		"password_reset",
		revokedHash,
		1,
		1,
		createdAt.Add(time.Hour),
		createdAt.Add(time.Minute),
		createdAt,
	)
	require.NoError(t, err)
	result, err = db.ExecContext(
		ctx,
		consumeSQL,
		consumeTime,
		"password_reset",
		revokedHash,
		consumeTime,
		consumeTime,
	)
	require.NoError(t, err)
	affected, err = result.RowsAffected()
	require.NoError(t, err)
	require.Zero(t, affected, "a revoked token must not be consumed")
}

func m8TokenID(t *testing.T, cfg config.DatabaseConfig, seed byte) any {
	t.Helper()

	if cfg.Driver == "postgres" {
		return fmt.Sprintf("%08x-0000-4000-8000-%012x", seed, seed)
	}
	if cfg.Driver == "mysql" {
		return bytes.Repeat([]byte{seed}, 16)
	}
	t.Fatalf("unsupported M8 database profile")
	return nil
}

func m8Hash(seed byte) []byte {
	return bytes.Repeat([]byte{seed}, 32)
}
