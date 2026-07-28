package migrations_test

import (
	"bytes"
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/korwlik23/go-api-starter/internal/platform/config"
	"github.com/stretchr/testify/require"
)

func m6AssertAuthAttemptBehavior(
	t *testing.T,
	ctx context.Context,
	db *sql.DB,
	cfg config.DatabaseConfig,
	seedOffset byte,
) {
	t.Helper()

	now := time.Now().UTC().Truncate(time.Microsecond)
	windowStart := now.Add(-time.Minute)
	windowExpiry := now.Add(5 * time.Minute)
	insertSQL := "INSERT INTO auth_attempts (" +
		"key_type, key_hash, failure_count, window_started_at, " +
		"window_expires_at, last_failed_at, locked_until" +
		") VALUES (" + migrationPlaceholders(cfg, 7) + ")"

	sharedHash := m6Hash(10 + seedOffset)
	_, err := db.ExecContext(
		ctx,
		insertSQL,
		"subject",
		sharedHash,
		1,
		windowStart,
		windowExpiry,
		now,
		nil,
	)
	require.NoError(t, err)
	_, err = db.ExecContext(
		ctx,
		insertSQL,
		"ip",
		sharedHash,
		1,
		windowStart,
		windowExpiry,
		now,
		nil,
	)
	require.NoError(t, err, "the same fingerprint must have independent subject and IP buckets")
	_, err = db.ExecContext(
		ctx,
		insertSQL,
		"subject",
		sharedHash,
		2,
		windowStart,
		windowExpiry,
		now,
		nil,
	)
	require.Error(t, err, "one key type and hash must identify exactly one bucket")

	unknownSubjectHash := m6Hash(11 + seedOffset)
	_, err = db.ExecContext(
		ctx,
		insertSQL,
		"subject",
		unknownSubjectHash,
		1,
		windowStart,
		windowExpiry,
		now,
		nil,
	)
	require.NoError(t, err, "unknown subjects must be throttled without a users foreign key")

	lockedHash := m6Hash(12 + seedOffset)
	lockedUntil := now.Add(2 * time.Minute)
	_, err = db.ExecContext(
		ctx,
		insertSQL,
		"ip",
		lockedHash,
		5,
		windowStart,
		windowExpiry,
		now,
		lockedUntil,
	)
	require.NoError(t, err)

	var failureCount int64
	var storedWindowStart time.Time
	var storedWindowExpiry time.Time
	var storedLastFailure time.Time
	var storedLock sql.NullTime
	var createdAt time.Time
	var updatedAt time.Time
	err = db.QueryRowContext(
		ctx,
		"SELECT failure_count, window_started_at, window_expires_at, "+
			"last_failed_at, locked_until, created_at, updated_at "+
			"FROM auth_attempts WHERE key_type = "+migrationPlaceholder(cfg, 1)+
			" AND key_hash = "+migrationPlaceholder(cfg, 2),
		"ip",
		lockedHash,
	).Scan(
		&failureCount,
		&storedWindowStart,
		&storedWindowExpiry,
		&storedLastFailure,
		&storedLock,
		&createdAt,
		&updatedAt,
	)
	require.NoError(t, err)
	require.Equal(t, int64(5), failureCount)
	require.WithinDuration(t, windowStart, storedWindowStart, time.Microsecond)
	require.WithinDuration(t, windowExpiry, storedWindowExpiry, time.Microsecond)
	require.WithinDuration(t, now, storedLastFailure, time.Microsecond)
	require.True(t, storedLock.Valid)
	require.WithinDuration(t, lockedUntil, storedLock.Time, time.Microsecond)
	require.False(t, createdAt.IsZero())
	require.False(t, updatedAt.IsZero())

	m6AssertInvalidAuthAttempts(
		t,
		ctx,
		db,
		cfg,
		insertSQL,
		windowStart,
		windowExpiry,
		now,
		seedOffset,
	)
}

func m6AssertInvalidAuthAttempts(
	t *testing.T,
	ctx context.Context,
	db *sql.DB,
	cfg config.DatabaseConfig,
	insertSQL string,
	windowStart time.Time,
	windowExpiry time.Time,
	lastFailure time.Time,
	seedOffset byte,
) {
	t.Helper()

	tests := []struct {
		name         string
		keyType      string
		failureCount int64
		windowStart  time.Time
		windowExpiry time.Time
		lastFailure  time.Time
		lockedUntil  any
	}{
		{
			name:         "unknown key type",
			keyType:      "device",
			failureCount: 1,
			windowStart:  windowStart,
			windowExpiry: windowExpiry,
			lastFailure:  lastFailure,
		},
		{
			name:         "case-folded key type",
			keyType:      "SUBJECT",
			failureCount: 1,
			windowStart:  windowStart,
			windowExpiry: windowExpiry,
			lastFailure:  lastFailure,
		},
		{
			name:         "zero counter",
			keyType:      "subject",
			failureCount: 0,
			windowStart:  windowStart,
			windowExpiry: windowExpiry,
			lastFailure:  lastFailure,
		},
		{
			name:         "negative counter",
			keyType:      "ip",
			failureCount: -1,
			windowStart:  windowStart,
			windowExpiry: windowExpiry,
			lastFailure:  lastFailure,
		},
		{
			name:         "window starts after failure",
			keyType:      "subject",
			failureCount: 1,
			windowStart:  lastFailure.Add(time.Second),
			windowExpiry: windowExpiry,
			lastFailure:  lastFailure,
		},
		{
			name:         "window has no duration",
			keyType:      "subject",
			failureCount: 1,
			windowStart:  lastFailure,
			windowExpiry: lastFailure,
			lastFailure:  lastFailure,
		},
		{
			name:         "failure occurs at window expiry",
			keyType:      "ip",
			failureCount: 1,
			windowStart:  windowStart,
			windowExpiry: lastFailure,
			lastFailure:  lastFailure,
		},
		{
			name:         "lock does not outlive failure",
			keyType:      "subject",
			failureCount: 5,
			windowStart:  windowStart,
			windowExpiry: windowExpiry,
			lastFailure:  lastFailure,
			lockedUntil:  lastFailure,
		},
	}
	for index, test := range tests {
		_, err := db.ExecContext(
			ctx,
			insertSQL,
			test.keyType,
			m6Hash(byte(30+index)+seedOffset),
			test.failureCount,
			test.windowStart,
			test.windowExpiry,
			test.lastFailure,
			test.lockedUntil,
		)
		require.Error(t, err, test.name)
	}

	for index, hash := range [][]byte{
		bytes.Repeat([]byte{70 + seedOffset}, 31),
		bytes.Repeat([]byte{71 + seedOffset}, 33),
	} {
		_, err := db.ExecContext(
			ctx,
			insertSQL,
			"subject",
			hash,
			1,
			windowStart,
			windowExpiry,
			lastFailure,
			nil,
		)
		require.Error(t, err, "fingerprint length boundary %d must be rejected", index)
	}
}

func m6Hash(seed byte) []byte {
	return bytes.Repeat([]byte{seed}, 32)
}
