package gorm

import (
	"context"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	identitydomain "github.com/korwlik23/go-api-starter/internal/modules/identity/domain"
	"github.com/stretchr/testify/require"
	gormdb "gorm.io/gorm"
)

func TestMFARepositorySavesNewPendingEncryptedEnvelope(t *testing.T) {
	t.Parallel()

	db, mock := sessionRepositoryDatabase(t)
	repository, err := NewMFARepository(db)
	require.NoError(t, err)
	now := time.Date(2026, time.July, 30, 7, 0, 0, 0, time.UTC)
	enrollment := identitydomain.MFAEnrollment{
		ID:         "018f1f64-7b2a-7c10-8bf2-3c40d5f67585",
		UserID:     "018f1f64-7b2a-7c10-8bf2-3c40d5f67586",
		State:      identitydomain.MFAEnrollmentPending,
		KeyVersion: 2,
		Nonce:      []byte("123456789012"),
		Ciphertext: []byte("encrypted-secret-contents"),
		ExpiresAt:  now.Add(10 * time.Minute),
		CreatedAt:  now,
	}

	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT .* FROM "mfa_totp_credentials".*FOR UPDATE`).
		WillReturnError(gormdb.ErrRecordNotFound)
	mock.ExpectExec(`INSERT INTO "mfa_totp_credentials"`).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	require.NoError(t, repository.SavePending(context.Background(), enrollment))
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestMFARepositoryRejectsActiveEnrollmentReplacement(t *testing.T) {
	t.Parallel()

	db, mock := sessionRepositoryDatabase(t)
	repository, err := NewMFARepository(db)
	require.NoError(t, err)
	now := time.Date(2026, time.July, 30, 7, 0, 0, 0, time.UTC)
	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT .* FROM "mfa_totp_credentials".*FOR UPDATE`).
		WillReturnRows(sqlmock.NewRows([]string{
			"user_id", "enrollment_id", "confirmed_at", "enrollment_expires_at",
		}).AddRow(
			"018f1f64-7b2a-7c10-8bf2-3c40d5f67586",
			"018f1f64-7b2a-7c10-8bf2-3c40d5f67584",
			now.Add(-time.Hour),
			nil,
		))
	mock.ExpectRollback()

	err = repository.SavePending(context.Background(), identitydomain.MFAEnrollment{
		ID:         "018f1f64-7b2a-7c10-8bf2-3c40d5f67585",
		UserID:     "018f1f64-7b2a-7c10-8bf2-3c40d5f67586",
		State:      identitydomain.MFAEnrollmentPending,
		KeyVersion: 2,
		Nonce:      []byte("123456789012"),
		Ciphertext: []byte("encrypted-secret-contents"),
		ExpiresAt:  now.Add(10 * time.Minute),
		CreatedAt:  now,
	})

	require.ErrorIs(t, err, ErrMFAEnrollmentConflict)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestMFARepositoryLoadsActiveEnvelope(t *testing.T) {
	t.Parallel()

	db, mock := sessionRepositoryDatabase(t)
	repository, err := NewMFARepository(db)
	require.NoError(t, err)
	userID := "018f1f64-7b2a-7c10-8bf2-3c40d5f67586"
	lastStep := int64(123)
	mock.ExpectQuery(`SELECT .* FROM "mfa_totp_credentials"`).
		WillReturnRows(sqlmock.NewRows([]string{
			"user_id", "enrollment_id", "secret_key_version", "secret_nonce",
			"secret_ciphertext", "enrollment_expires_at", "confirmed_at",
			"last_used_step", "created_at",
		}).AddRow(
			userID,
			"018f1f64-7b2a-7c10-8bf2-3c40d5f67585",
			int64(2),
			[]byte("123456789012"),
			[]byte("encrypted-secret-contents"),
			nil,
			time.Now(),
			lastStep,
			time.Now().Add(-time.Hour),
		))

	enrollment, err := repository.LoadActive(context.Background(), userID)

	require.NoError(t, err)
	require.Equal(t, identitydomain.MFAEnrollmentActive, enrollment.State)
	require.Equal(t, lastStep, *enrollment.LastUsedStep)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestMFARepositoryCompletesTOTPAndSessionRotationAtomically(t *testing.T) {
	t.Parallel()

	db, mock := sessionRepositoryDatabase(t)
	repository, err := NewMFARepository(db)
	require.NoError(t, err)
	now := time.Date(2026, time.July, 30, 7, 0, 0, 0, time.UTC)
	userID := "018f1f64-7b2a-7c10-8bf2-3c40d5f67586"
	version := int64(9)
	replacement := identitydomain.Session{
		ID:                   "018f1f64-7b2a-7c10-8bf2-3c40d5f67587",
		TokenHash:            identitydomain.TokenHash{0x31},
		CSRFTokenHash:        identitydomain.TokenHash{0x32},
		State:                identitydomain.SessionStateAuthenticated,
		UserID:               &userID,
		AuthorizationVersion: &version,
		IdleExpiresAt:        now.Add(time.Hour),
		AbsoluteExpiresAt:    now.Add(12 * time.Hour),
		CreatedAt:            now,
	}
	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta(
		`UPDATE "mfa_totp_credentials" SET "last_used_step"=$1,"updated_at"=$2 WHERE enrollment_id = $3 AND confirmed_at IS NOT NULL AND (last_used_step IS NULL OR last_used_step < $4)`,
	)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	pendingHash := identitydomain.TokenHash{0x11}
	mock.ExpectExec(regexp.QuoteMeta(
		`UPDATE "sessions" SET "revoked_at"=$1 WHERE token_hash = $2 AND state = $3 AND revoked_at IS NULL`,
	)).
		WithArgs(now, pendingHash[:], string(identitydomain.SessionStateMFAPending)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`INSERT INTO "sessions"`).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	err = repository.CompleteTOTP(
		context.Background(),
		"018f1f64-7b2a-7c10-8bf2-3c40d5f67585",
		124,
		pendingHash,
		replacement,
	)

	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}
