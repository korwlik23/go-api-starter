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

func m7AssertMFABehavior(
	t *testing.T,
	ctx context.Context,
	db *sql.DB,
	cfg config.DatabaseConfig,
	seedOffset byte,
) {
	t.Helper()

	userID := m3UserID(t, cfg, 20+seedOffset)
	m2InsertUser(t, ctx, db, cfg, userID, fmt.Sprintf("mfa-%d@example.test", seedOffset))
	now := time.Now().UTC().Truncate(time.Microsecond)
	enrollmentExpiry := now.Add(10 * time.Minute)
	ciphertext := bytes.Repeat([]byte{30 + seedOffset}, 36)
	nonce := bytes.Repeat([]byte{31 + seedOffset}, 12)
	rawSecretMarker := []byte("JBSWY3DPEHPK3PXP")
	insertCredentialSQL := "INSERT INTO mfa_totp_credentials (" +
		"user_id, secret_envelope_version, secret_key_version, secret_nonce, " +
		"secret_ciphertext, enrollment_expires_at" +
		") VALUES (" + migrationPlaceholders(cfg, 6) + ")"
	_, err := db.ExecContext(
		ctx,
		insertCredentialSQL,
		userID,
		1,
		1,
		nonce,
		ciphertext,
		enrollmentExpiry,
	)
	require.NoError(t, err)
	require.False(t, bytes.Contains(ciphertext, rawSecretMarker))

	_, err = db.ExecContext(
		ctx,
		insertCredentialSQL,
		userID,
		1,
		1,
		bytes.Repeat([]byte{32 + seedOffset}, 12),
		bytes.Repeat([]byte{33 + seedOffset}, 36),
		enrollmentExpiry,
	)
	require.Error(t, err, "one user must have at most one TOTP credential")
	_, err = db.ExecContext(
		ctx,
		insertCredentialSQL,
		m3UserID(t, cfg, 99+seedOffset),
		1,
		1,
		bytes.Repeat([]byte{34 + seedOffset}, 12),
		ciphertext,
		enrollmentExpiry,
	)
	require.Error(t, err, "a TOTP credential for an unknown user must be rejected")

	var storedCiphertext []byte
	var storedNonce []byte
	var envelopeVersion int16
	var keyVersion int64
	var storedEnrollmentExpiry sql.NullTime
	var confirmedAt sql.NullTime
	var lastUsedStep sql.NullInt64
	var createdAt time.Time
	var updatedAt time.Time
	err = db.QueryRowContext(
		ctx,
		"SELECT secret_ciphertext, secret_nonce, secret_envelope_version, "+
			"secret_key_version, enrollment_expires_at, confirmed_at, last_used_step, "+
			"created_at, updated_at "+
			"FROM mfa_totp_credentials WHERE user_id = "+migrationPlaceholder(cfg, 1),
		userID,
	).Scan(
		&storedCiphertext,
		&storedNonce,
		&envelopeVersion,
		&keyVersion,
		&storedEnrollmentExpiry,
		&confirmedAt,
		&lastUsedStep,
		&createdAt,
		&updatedAt,
	)
	require.NoError(t, err)
	require.Equal(t, ciphertext, storedCiphertext)
	require.Equal(t, nonce, storedNonce)
	require.Equal(t, int16(1), envelopeVersion)
	require.Equal(t, int64(1), keyVersion)
	require.True(t, storedEnrollmentExpiry.Valid)
	require.WithinDuration(t, enrollmentExpiry, storedEnrollmentExpiry.Time, time.Microsecond)
	require.False(t, bytes.Contains(storedCiphertext, rawSecretMarker))
	require.False(t, confirmedAt.Valid)
	require.False(t, lastUsedStep.Valid)
	require.False(t, createdAt.IsZero())
	require.False(t, updatedAt.IsZero())

	confirmationTime := createdAt.Add(time.Second)
	const confirmationStep int64 = 123456
	_, err = db.ExecContext(
		ctx,
		"UPDATE mfa_totp_credentials SET enrollment_expires_at = NULL, confirmed_at = "+
			migrationPlaceholder(cfg, 1)+", last_used_step = "+migrationPlaceholder(cfg, 2)+
			", updated_at = "+migrationPlaceholder(cfg, 3)+
			" WHERE user_id = "+migrationPlaceholder(cfg, 4),
		confirmationTime,
		confirmationStep,
		confirmationTime,
		userID,
	)
	require.NoError(t, err)

	m7AssertTOTPReplayState(t, ctx, db, cfg, userID, confirmationTime, confirmationStep)
	m7AssertRecoveryCodeBehavior(t, ctx, db, cfg, userID, seedOffset)
	m7AssertInvalidMFAState(t, ctx, db, cfg, insertCredentialSQL, seedOffset)

	_, err = db.ExecContext(
		ctx,
		"DELETE FROM users WHERE id = "+migrationPlaceholder(cfg, 1),
		userID,
	)
	require.NoError(t, err)
	for _, table := range []string{m7TOTPTable, m7RecoveryCodesTable} {
		var count int
		require.NoError(t, db.QueryRowContext(
			ctx,
			"SELECT COUNT(*) FROM "+table+" WHERE user_id = "+migrationPlaceholder(cfg, 1),
			userID,
		).Scan(&count))
		require.Zero(t, count, "MFA state must be removed with its user")
	}
}

func m7AssertTOTPReplayState(
	t *testing.T,
	ctx context.Context,
	db *sql.DB,
	cfg config.DatabaseConfig,
	userID any,
	usedAt time.Time,
	lastAcceptedStep int64,
) {
	t.Helper()

	acceptedStep := lastAcceptedStep + 1
	updateSQL := "UPDATE mfa_totp_credentials SET last_used_step = " +
		migrationPlaceholder(cfg, 1) + ", updated_at = " + migrationPlaceholder(cfg, 2) +
		" WHERE user_id = " + migrationPlaceholder(cfg, 3) +
		" AND confirmed_at IS NOT NULL AND (last_used_step IS NULL OR last_used_step < " +
		migrationPlaceholder(cfg, 4) + ")"
	result, err := db.ExecContext(ctx, updateSQL, acceptedStep, usedAt, userID, acceptedStep)
	require.NoError(t, err)
	affected, err := result.RowsAffected()
	require.NoError(t, err)
	require.Equal(t, int64(1), affected)

	for _, replayedStep := range []int64{acceptedStep, acceptedStep - 1} {
		result, err = db.ExecContext(ctx, updateSQL, replayedStep, usedAt, userID, replayedStep)
		require.NoError(t, err)
		affected, err = result.RowsAffected()
		require.NoError(t, err)
		require.Zero(t, affected, "a consumed TOTP step must not be accepted again")
	}
}

func m7AssertRecoveryCodeBehavior(
	t *testing.T,
	ctx context.Context,
	db *sql.DB,
	cfg config.DatabaseConfig,
	userID any,
	seedOffset byte,
) {
	t.Helper()

	insertSQL := "INSERT INTO mfa_recovery_codes (" +
		"user_id, code_hash, hash_version, hash_key_version" +
		") VALUES (" + migrationPlaceholders(cfg, 4) + ")"
	firstHash := m7RecoveryCodeHash(50 + seedOffset)
	secondHash := m7RecoveryCodeHash(51 + seedOffset)
	for _, codeHash := range [][]byte{firstHash, secondHash} {
		_, err := db.ExecContext(ctx, insertSQL, userID, codeHash, 1, 1)
		require.NoError(t, err)
	}
	_, err := db.ExecContext(ctx, insertSQL, userID, firstHash, 1, 1)
	require.Error(t, err, "the same recovery code hash cannot be issued twice")
	_, err = db.ExecContext(
		ctx,
		insertSQL,
		m3UserID(t, cfg, 98+seedOffset),
		m7RecoveryCodeHash(52+seedOffset),
		1,
		1,
	)
	require.Error(t, err, "recovery codes require an existing TOTP credential")

	var recoveryCodeCreatedAt time.Time
	require.NoError(t, db.QueryRowContext(
		ctx,
		"SELECT created_at FROM mfa_recovery_codes WHERE user_id = "+
			migrationPlaceholder(cfg, 1)+" AND code_hash = "+migrationPlaceholder(cfg, 2),
		userID,
		firstHash,
	).Scan(&recoveryCodeCreatedAt))
	consumedAt := recoveryCodeCreatedAt.Add(time.Second)
	consumeSQL := "UPDATE mfa_recovery_codes SET consumed_at = " +
		migrationPlaceholder(cfg, 1) + " WHERE user_id = " + migrationPlaceholder(cfg, 2) +
		" AND code_hash = " + migrationPlaceholder(cfg, 3) + " AND consumed_at IS NULL"
	result, err := db.ExecContext(ctx, consumeSQL, consumedAt, userID, firstHash)
	require.NoError(t, err)
	affected, err := result.RowsAffected()
	require.NoError(t, err)
	require.Equal(t, int64(1), affected)
	result, err = db.ExecContext(ctx, consumeSQL, consumedAt.Add(time.Second), userID, firstHash)
	require.NoError(t, err)
	affected, err = result.RowsAffected()
	require.NoError(t, err)
	require.Zero(t, affected, "a consumed recovery code must not be accepted again")

	for index, codeHash := range [][]byte{
		bytes.Repeat([]byte{60 + seedOffset}, 31),
		bytes.Repeat([]byte{61 + seedOffset}, 33),
	} {
		_, err = db.ExecContext(ctx, insertSQL, userID, codeHash, 1, 1)
		require.Error(t, err, "recovery code hash boundary %d must be rejected", index)
	}
	for index, versions := range [][2]int{{0, 1}, {1, 0}} {
		_, err = db.ExecContext(
			ctx,
			insertSQL,
			userID,
			m7RecoveryCodeHash(byte(63+index)+seedOffset),
			versions[0],
			versions[1],
		)
		require.Error(t, err, "non-positive recovery hash metadata %d must be rejected", index)
	}
	_, err = db.ExecContext(
		ctx,
		"INSERT INTO mfa_recovery_codes (user_id, code_hash, hash_version, "+
			"hash_key_version, consumed_at, created_at) VALUES ("+
			migrationPlaceholders(cfg, 6)+")",
		userID,
		m7RecoveryCodeHash(62+seedOffset),
		1,
		1,
		recoveryCodeCreatedAt.Add(-time.Second),
		recoveryCodeCreatedAt,
	)
	require.Error(t, err, "a recovery code cannot be consumed before it is created")
}

func m7AssertInvalidMFAState(
	t *testing.T,
	ctx context.Context,
	db *sql.DB,
	cfg config.DatabaseConfig,
	insertCredentialSQL string,
	seedOffset byte,
) {
	t.Helper()

	now := time.Now().UTC().Truncate(time.Microsecond)
	tests := []struct {
		name             string
		ciphertext       []byte
		nonce            []byte
		envelopeVersion  int
		keyVersion       int64
		enrollmentExpiry any
	}{
		{name: "ciphertext too short", ciphertext: bytes.Repeat([]byte{70}, 16), nonce: bytes.Repeat([]byte{71}, 12), envelopeVersion: 1, keyVersion: 1, enrollmentExpiry: now.Add(time.Minute)},
		{name: "ciphertext too long", ciphertext: bytes.Repeat([]byte{72}, 513), nonce: bytes.Repeat([]byte{73}, 12), envelopeVersion: 1, keyVersion: 1, enrollmentExpiry: now.Add(time.Minute)},
		{name: "nonce too short", ciphertext: bytes.Repeat([]byte{74}, 36), nonce: bytes.Repeat([]byte{75}, 11), envelopeVersion: 1, keyVersion: 1, enrollmentExpiry: now.Add(time.Minute)},
		{name: "nonce too long", ciphertext: bytes.Repeat([]byte{76}, 36), nonce: bytes.Repeat([]byte{77}, 13), envelopeVersion: 1, keyVersion: 1, enrollmentExpiry: now.Add(time.Minute)},
		{name: "unsupported envelope version", ciphertext: bytes.Repeat([]byte{78}, 36), nonce: bytes.Repeat([]byte{79}, 12), envelopeVersion: 2, keyVersion: 1, enrollmentExpiry: now.Add(time.Minute)},
		{name: "zero key version", ciphertext: bytes.Repeat([]byte{80}, 36), nonce: bytes.Repeat([]byte{81}, 12), envelopeVersion: 1, keyVersion: 0, enrollmentExpiry: now.Add(time.Minute)},
		{name: "missing enrollment expiry", ciphertext: bytes.Repeat([]byte{82}, 36), nonce: bytes.Repeat([]byte{83}, 12), envelopeVersion: 1, keyVersion: 1, enrollmentExpiry: nil},
	}
	for index, test := range tests {
		userID := m3UserID(t, cfg, byte(30+index)+seedOffset)
		m2InsertUser(t, ctx, db, cfg, userID, fmt.Sprintf("invalid-mfa-%d-%d@example.test", seedOffset, index))
		_, err := db.ExecContext(
			ctx,
			insertCredentialSQL,
			userID,
			test.envelopeVersion,
			test.keyVersion,
			test.nonce,
			test.ciphertext,
			test.enrollmentExpiry,
		)
		require.Error(t, err, test.name)
	}

	duplicateNonceUserID := m3UserID(t, cfg, 38+seedOffset)
	m2InsertUser(t, ctx, db, cfg, duplicateNonceUserID, fmt.Sprintf("duplicate-mfa-nonce-%d@example.test", seedOffset))
	_, err := db.ExecContext(
		ctx,
		insertCredentialSQL,
		duplicateNonceUserID,
		1,
		1,
		bytes.Repeat([]byte{31 + seedOffset}, 12),
		bytes.Repeat([]byte{84 + seedOffset}, 36),
		now.Add(time.Minute),
	)
	require.Error(t, err, "an encryption key version must never reuse a nonce")

	createdAt := now
	for index, state := range []struct {
		name             string
		enrollmentExpiry any
		confirmedAt      any
		lastUsedStep     any
	}{
		{name: "pending credential with used step", enrollmentExpiry: createdAt.Add(time.Minute), confirmedAt: nil, lastUsedStep: 1},
		{name: "active credential without used step", enrollmentExpiry: nil, confirmedAt: createdAt.Add(time.Second), lastUsedStep: nil},
		{name: "active credential with zero used step", enrollmentExpiry: nil, confirmedAt: createdAt.Add(time.Second), lastUsedStep: 0},
		{name: "confirmation predates creation", enrollmentExpiry: nil, confirmedAt: createdAt.Add(-time.Second), lastUsedStep: 1},
		{name: "expired pending enrollment", enrollmentExpiry: createdAt, confirmedAt: nil, lastUsedStep: nil},
		{name: "confirmed pending enrollment", enrollmentExpiry: createdAt.Add(time.Minute), confirmedAt: createdAt.Add(time.Second), lastUsedStep: 1},
	} {
		userID := m3UserID(t, cfg, byte(40+index)+seedOffset)
		m2InsertUser(t, ctx, db, cfg, userID, fmt.Sprintf("invalid-mfa-state-%d-%d@example.test", seedOffset, index))
		_, err := db.ExecContext(
			ctx,
			"INSERT INTO mfa_totp_credentials (user_id, secret_envelope_version, "+
				"secret_key_version, secret_nonce, secret_ciphertext, enrollment_expires_at, "+
				"confirmed_at, last_used_step, created_at, updated_at) VALUES ("+
				migrationPlaceholders(cfg, 10)+")",
			userID,
			1,
			2,
			bytes.Repeat([]byte{90 + byte(index) + seedOffset}, 12),
			bytes.Repeat([]byte{100 + byte(index) + seedOffset}, 36),
			state.enrollmentExpiry,
			state.confirmedAt,
			state.lastUsedStep,
			createdAt,
			createdAt,
		)
		require.Error(t, err, state.name)
	}
}

func m7RecoveryCodeHash(seed byte) []byte {
	return bytes.Repeat([]byte{seed}, 32)
}
