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

func m5AssertSessionBehavior(
	t *testing.T,
	ctx context.Context,
	db *sql.DB,
	cfg config.DatabaseConfig,
	seedOffset byte,
) {
	t.Helper()

	userID := m3UserID(t, cfg, 10+seedOffset)
	m2InsertUser(
		t,
		ctx,
		db,
		cfg,
		userID,
		fmt.Sprintf("session-%d@example.test", seedOffset),
	)
	now := time.Now().UTC().Truncate(time.Microsecond)
	idleExpiry := now.Add(15 * time.Minute)
	absoluteExpiry := now.Add(time.Hour)
	insertSQL := "INSERT INTO sessions (" +
		"id, token_hash, csrf_token_hash, state, user_id, " +
		"authorization_version, idle_expires_at, absolute_expires_at" +
		") VALUES (" + migrationPlaceholders(cfg, 8) + ")"
	authenticatedID := m5SessionID(t, cfg, 20+seedOffset)
	authenticatedTokenHash := m5Hash(30 + seedOffset)
	_, err := db.ExecContext(
		ctx,
		insertSQL,
		authenticatedID,
		authenticatedTokenHash,
		m5Hash(31+seedOffset),
		"authenticated",
		userID,
		1,
		idleExpiry,
		absoluteExpiry,
	)
	require.NoError(t, err)

	_, err = db.ExecContext(
		ctx,
		insertSQL,
		m5SessionID(t, cfg, 21+seedOffset),
		authenticatedTokenHash,
		m5Hash(32+seedOffset),
		"authenticated",
		userID,
		1,
		idleExpiry,
		absoluteExpiry,
	)
	require.Error(t, err, "duplicate token hash must be rejected")
	_, err = db.ExecContext(
		ctx,
		insertSQL,
		authenticatedID,
		m5Hash(33+seedOffset),
		m5Hash(34+seedOffset),
		"authenticated",
		userID,
		1,
		idleExpiry,
		absoluteExpiry,
	)
	require.Error(t, err, "duplicate session ID must be rejected")

	anonymousID := m5SessionID(t, cfg, 22+seedOffset)
	_, err = db.ExecContext(
		ctx,
		insertSQL,
		anonymousID,
		m5Hash(35+seedOffset),
		m5Hash(36+seedOffset),
		"anonymous",
		nil,
		nil,
		idleExpiry,
		absoluteExpiry,
	)
	require.NoError(t, err, "anonymous session must not require a user")
	_, err = db.ExecContext(
		ctx,
		insertSQL,
		m5SessionID(t, cfg, 23+seedOffset),
		m5Hash(37+seedOffset),
		m5Hash(38+seedOffset),
		"mfa_pending",
		userID,
		1,
		idleExpiry,
		absoluteExpiry,
	)
	require.NoError(t, err, "MFA-pending session must retain its principal snapshot")

	m5AssertInvalidSessionStates(
		t,
		ctx,
		db,
		cfg,
		insertSQL,
		userID,
		now,
		idleExpiry,
		absoluteExpiry,
		seedOffset,
	)

	var state string
	var authorizationVersion sql.NullInt64
	var revokedAt sql.NullTime
	var createdAt time.Time
	err = db.QueryRowContext(
		ctx,
		"SELECT state, authorization_version, revoked_at, created_at "+
			"FROM sessions WHERE id = "+migrationPlaceholder(cfg, 1),
		authenticatedID,
	).Scan(&state, &authorizationVersion, &revokedAt, &createdAt)
	require.NoError(t, err)
	require.Equal(t, "authenticated", state)
	require.Equal(t, int64(1), authorizationVersion.Int64)
	require.True(t, authorizationVersion.Valid)
	require.False(t, revokedAt.Valid)
	require.False(t, createdAt.IsZero())
	_, err = db.ExecContext(
		ctx,
		"UPDATE sessions SET revoked_at = "+migrationPlaceholder(cfg, 1)+
			" WHERE id = "+migrationPlaceholder(cfg, 2),
		createdAt.Add(-time.Second),
		authenticatedID,
	)
	require.Error(t, err, "revocation must not predate session creation")

	revokedTime := now.Add(5 * time.Minute)
	_, err = db.ExecContext(
		ctx,
		"UPDATE sessions SET revoked_at = "+migrationPlaceholder(cfg, 1)+
			" WHERE id = "+migrationPlaceholder(cfg, 2),
		revokedTime,
		authenticatedID,
	)
	require.NoError(t, err)
	require.NoError(t, db.QueryRowContext(
		ctx,
		"SELECT revoked_at FROM sessions WHERE id = "+migrationPlaceholder(cfg, 1),
		authenticatedID,
	).Scan(&revokedAt))
	require.True(t, revokedAt.Valid)
	require.WithinDuration(t, revokedTime, revokedAt.Time, time.Microsecond)

	_, err = db.ExecContext(
		ctx,
		"DELETE FROM users WHERE id = "+migrationPlaceholder(cfg, 1),
		userID,
	)
	require.NoError(t, err)
	var principalSessionCount int
	require.NoError(t, db.QueryRowContext(
		ctx,
		"SELECT COUNT(*) FROM sessions WHERE user_id = "+migrationPlaceholder(cfg, 1),
		userID,
	).Scan(&principalSessionCount))
	require.Zero(t, principalSessionCount)
	var anonymousSessionCount int
	require.NoError(t, db.QueryRowContext(
		ctx,
		"SELECT COUNT(*) FROM sessions WHERE id = "+migrationPlaceholder(cfg, 1),
		anonymousID,
	).Scan(&anonymousSessionCount))
	require.Equal(t, 1, anonymousSessionCount)
}

func m5AssertInvalidSessionStates(
	t *testing.T,
	ctx context.Context,
	db *sql.DB,
	cfg config.DatabaseConfig,
	insertSQL string,
	userID any,
	now time.Time,
	idleExpiry time.Time,
	absoluteExpiry time.Time,
	seedOffset byte,
) {
	t.Helper()

	tests := []struct {
		name                 string
		state                string
		userID               any
		authorizationVersion any
		idleExpiry           time.Time
		absoluteExpiry       time.Time
	}{
		{
			name:                 "unknown state",
			state:                "future-state",
			userID:               userID,
			authorizationVersion: 1,
			idleExpiry:           idleExpiry,
			absoluteExpiry:       absoluteExpiry,
		},
		{
			name:                 "anonymous principal",
			state:                "anonymous",
			userID:               userID,
			authorizationVersion: 1,
			idleExpiry:           idleExpiry,
			absoluteExpiry:       absoluteExpiry,
		},
		{
			name:                 "authenticated without principal",
			state:                "authenticated",
			userID:               nil,
			authorizationVersion: nil,
			idleExpiry:           idleExpiry,
			absoluteExpiry:       absoluteExpiry,
		},
		{
			name:                 "authenticated without authorization version",
			state:                "authenticated",
			userID:               userID,
			authorizationVersion: nil,
			idleExpiry:           idleExpiry,
			absoluteExpiry:       absoluteExpiry,
		},
		{
			name:                 "non-positive authorization version",
			state:                "authenticated",
			userID:               userID,
			authorizationVersion: 0,
			idleExpiry:           idleExpiry,
			absoluteExpiry:       absoluteExpiry,
		},
		{
			name:                 "idle expiry after absolute expiry",
			state:                "authenticated",
			userID:               userID,
			authorizationVersion: 1,
			idleExpiry:           absoluteExpiry.Add(time.Second),
			absoluteExpiry:       absoluteExpiry,
		},
		{
			name:                 "absolute expiry before creation",
			state:                "authenticated",
			userID:               userID,
			authorizationVersion: 1,
			idleExpiry:           now.Add(-2 * time.Hour),
			absoluteExpiry:       now.Add(-time.Hour),
		},
	}
	for index, test := range tests {
		_, err := db.ExecContext(
			ctx,
			insertSQL,
			m5SessionID(t, cfg, byte(40+index)+seedOffset),
			m5Hash(byte(50+index)+seedOffset),
			m5Hash(byte(60+index)+seedOffset),
			test.state,
			test.userID,
			test.authorizationVersion,
			test.idleExpiry,
			test.absoluteExpiry,
		)
		require.Error(t, err, test.name)
	}

	_, err := db.ExecContext(
		ctx,
		insertSQL,
		m5SessionID(t, cfg, 75+seedOffset),
		bytes.Repeat([]byte{75 + seedOffset}, 31),
		m5Hash(76+seedOffset),
		"anonymous",
		nil,
		nil,
		idleExpiry,
		absoluteExpiry,
	)
	require.Error(t, err, "session token hash must be exactly 32 bytes")
	_, err = db.ExecContext(
		ctx,
		insertSQL,
		m5SessionID(t, cfg, 76+seedOffset),
		m5Hash(77+seedOffset),
		bytes.Repeat([]byte{78 + seedOffset}, 31),
		"anonymous",
		nil,
		nil,
		idleExpiry,
		absoluteExpiry,
	)
	require.Error(t, err, "CSRF token hash must be exactly 32 bytes")
	_, err = db.ExecContext(
		ctx,
		insertSQL,
		m5SessionID(t, cfg, 77+seedOffset),
		bytes.Repeat([]byte{79 + seedOffset}, 33),
		m5Hash(80+seedOffset),
		"anonymous",
		nil,
		nil,
		idleExpiry,
		absoluteExpiry,
	)
	require.Error(t, err, "session token hash must reject values longer than 32 bytes")
	_, err = db.ExecContext(
		ctx,
		insertSQL,
		m5SessionID(t, cfg, 78+seedOffset),
		m5Hash(81+seedOffset),
		bytes.Repeat([]byte{82 + seedOffset}, 33),
		"anonymous",
		nil,
		nil,
		idleExpiry,
		absoluteExpiry,
	)
	require.Error(t, err, "CSRF token hash must reject values longer than 32 bytes")

	_, err = db.ExecContext(
		ctx,
		insertSQL,
		m5SessionID(t, cfg, 70+seedOffset),
		m5Hash(70+seedOffset),
		m5Hash(71+seedOffset),
		"authenticated",
		m3UserID(t, cfg, 99+seedOffset),
		1,
		idleExpiry,
		absoluteExpiry,
	)
	require.Error(t, err, "session for an unknown user must be rejected")
}

func m5SessionID(t *testing.T, cfg config.DatabaseConfig, seed byte) any {
	t.Helper()

	if cfg.Driver == "postgres" {
		return fmt.Sprintf("%08x-0000-4000-8000-%012x", seed, seed)
	}
	if cfg.Driver == "mysql" {
		return bytes.Repeat([]byte{seed}, 16)
	}
	t.Fatalf("unsupported M5 database profile")
	return nil
}

func m5Hash(seed byte) []byte {
	return bytes.Repeat([]byte{seed}, 32)
}
