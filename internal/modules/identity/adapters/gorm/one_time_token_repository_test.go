package gorm

import (
	"context"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	identitydomain "github.com/korwlik23/go-api-starter/internal/modules/identity/domain"
	"github.com/korwlik23/go-api-starter/internal/modules/identity/ports"
	"github.com/stretchr/testify/require"
)

func TestOneTimeTokenRepositoryIssuesTokenAndEncryptedDeliveryAtomically(t *testing.T) {
	t.Parallel()

	db, mock := sessionRepositoryDatabase(t)
	repository, err := NewOneTimeTokenRepository(db)
	require.NoError(t, err)
	now := time.Date(2026, time.July, 30, 8, 0, 0, 0, time.UTC)
	token := identitydomain.OneTimeToken{
		ID:             "018f1f64-7b2a-7c10-8bf2-3c40d5f67585",
		UserID:         "018f1f64-7b2a-7c10-8bf2-3c40d5f67586",
		Purpose:        identitydomain.OneTimeTokenPurposePasswordReset,
		TokenHash:      identitydomain.TokenHash{0x42},
		HashVersion:    1,
		HashKeyVersion: 3,
		ExpiresAt:      now.Add(30 * time.Minute),
		CreatedAt:      now,
	}
	delivery := identitydomain.EncryptedTokenDelivery{
		TokenID:    token.ID,
		KeyVersion: 4,
		Nonce:      []byte("123456789012"),
		Ciphertext: []byte("encrypted-recipient-and-token"),
		ExpiresAt:  token.ExpiresAt,
		CreatedAt:  now,
	}
	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT .* FROM "users".*FOR UPDATE`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(token.UserID))
	mock.ExpectExec(`UPDATE "one_time_tokens" SET "revoked_at"`).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`INSERT INTO "one_time_tokens"`).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(`INSERT INTO "one_time_token_deliveries"`).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	require.NoError(t, repository.Issue(context.Background(), token, delivery))
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestOneTimeTokenRepositoryFindsPasswordResetMetadata(t *testing.T) {
	t.Parallel()

	db, mock := sessionRepositoryDatabase(t)
	repository, err := NewOneTimeTokenRepository(db)
	require.NoError(t, err)
	tokenID := "018f1f64-7b2a-7c10-8bf2-3c40d5f67585"
	userID := "018f1f64-7b2a-7c10-8bf2-3c40d5f67586"
	mock.ExpectQuery(`SELECT .* FROM "one_time_tokens".*JOIN users`).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "user_id", "hash_version", "hash_key_version",
			"authorization_version", "mfa_required",
		}).AddRow(tokenID, userID, int16(1), int64(3), int64(8), true))

	metadata, err := repository.FindPasswordResetToken(context.Background(), tokenID)

	require.NoError(t, err)
	require.Equal(t, userID, metadata.UserID)
	require.Equal(t, int64(8), metadata.AuthorizationVersion)
	require.True(t, metadata.MFARequired)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestOneTimeTokenRepositoryCompletesPasswordResetTransaction(t *testing.T) {
	t.Parallel()

	db, mock := sessionRepositoryDatabase(t)
	repository, err := NewOneTimeTokenRepository(db)
	require.NoError(t, err)
	now := time.Date(2026, time.July, 30, 8, 0, 0, 0, time.UTC)
	userID := "018f1f64-7b2a-7c10-8bf2-3c40d5f67586"
	version := int64(9)
	command := ports.CompletePasswordResetCommand{
		TokenID:            "018f1f64-7b2a-7c10-8bf2-3c40d5f67585",
		CandidateHash:      identitydomain.TokenHash{0x42},
		HashVersion:        1,
		NewPasswordHash:    "new-argon2id-hash",
		CurrentSessionHash: identitydomain.TokenHash{0x11},
		Replacement: identitydomain.Session{
			ID:                   "018f1f64-7b2a-7c10-8bf2-3c40d5f67587",
			TokenHash:            identitydomain.TokenHash{0x31},
			CSRFTokenHash:        identitydomain.TokenHash{0x32},
			State:                identitydomain.SessionStateAuthenticated,
			UserID:               &userID,
			AuthorizationVersion: &version,
			IdleExpiresAt:        now.Add(time.Hour),
			AbsoluteExpiresAt:    now.Add(12 * time.Hour),
			CreatedAt:            now,
		},
		Now: now,
	}
	mock.ExpectBegin()
	mock.ExpectExec(`UPDATE "one_time_tokens" SET "consumed_at"`).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta(
		`UPDATE "password_credentials" SET "changed_at"=$1,"password_hash"=$2 WHERE user_id = $3`,
	)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`UPDATE "users" SET "authorization_version"`).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`UPDATE "sessions" SET "revoked_at"`).
		WillReturnResult(sqlmock.NewResult(0, 2))
	mock.ExpectExec(`UPDATE "one_time_tokens" SET "revoked_at"`).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`INSERT INTO "sessions"`).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	require.NoError(
		t,
		repository.CompletePasswordReset(context.Background(), command),
	)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestOneTimeTokenRepositoryRollsBackWhenTokenWasConsumed(t *testing.T) {
	t.Parallel()

	db, mock := sessionRepositoryDatabase(t)
	repository, err := NewOneTimeTokenRepository(db)
	require.NoError(t, err)
	now := time.Now()
	userID := "018f1f64-7b2a-7c10-8bf2-3c40d5f67586"
	version := int64(2)
	mock.ExpectBegin()
	mock.ExpectExec(`UPDATE "one_time_tokens" SET "consumed_at"`).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectRollback()

	err = repository.CompletePasswordReset(
		context.Background(),
		ports.CompletePasswordResetCommand{
			TokenID:         "018f1f64-7b2a-7c10-8bf2-3c40d5f67585",
			CandidateHash:   identitydomain.TokenHash{0x42},
			HashVersion:     1,
			NewPasswordHash: "new-argon2id-hash",
			Replacement: identitydomain.Session{
				UserID:               &userID,
				AuthorizationVersion: &version,
			},
			Now: now,
		},
	)

	require.ErrorIs(t, err, ports.ErrOneTimeTokenConflict)
	require.NoError(t, mock.ExpectationsWereMet())
}
