package gorm

import (
	"context"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	gormdb "gorm.io/gorm"
)

func TestM3C2PreviewRepositoryFindMapsBoundTokenMetadata(t *testing.T) {
	t.Parallel()

	db, mock := previewRepositoryDatabase(t)
	repository, err := NewPreviewRepository(db)
	require.NoError(t, err)
	createdAt := time.Date(2026, 8, 2, 12, 0, 0, 0, time.UTC)
	expiresAt := createdAt.Add(5 * time.Minute)
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "publishing_preview_tokens" WHERE id = $1 LIMIT $2`)).
		WithArgs(
			"018f1f64-7b2a-7c10-8bf2-3c40d5f67585", 1,
		).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "user_id", "account_id", "content_id", "translation_id", "revision_id",
			"token_hash", "hash_version", "hash_key_version", "audience", "expires_at", "consumed_at", "created_at",
		}).AddRow(
			"018f1f64-7b2a-7c10-8bf2-3c40d5f67585",
			"018f1f64-7b2a-7c10-8bf2-3c40d5f67581",
			"018f1f64-7b2a-7c10-8bf2-3c40d5f67580",
			"018f1f64-7b2a-7c10-8bf2-3c40d5f67582",
			"018f1f64-7b2a-7c10-8bf2-3c40d5f67583",
			"018f1f64-7b2a-7c10-8bf2-3c40d5f67584",
			func() []byte { value := make([]byte, 32); value[0] = 1; return value }(), 1, 1, "public", expiresAt, nil, createdAt,
		))

	token, err := repository.Find(context.Background(), "018f1f64-7b2a-7c10-8bf2-3c40d5f67585")
	require.NoError(t, err)
	require.Equal(t, "public", token.Audience)
	require.Equal(t, expiresAt, token.ExpiresAt)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestM3C2PreviewRepositoryConsumeIsAtomicAndSingleUse(t *testing.T) {
	t.Parallel()

	db, mock := previewRepositoryDatabase(t)
	repository, err := NewPreviewRepository(db)
	require.NoError(t, err)
	now := time.Date(2026, 8, 2, 12, 1, 0, 0, time.UTC)
	var hash [32]byte
	hash[0] = 1
	mock.ExpectBegin()
	mock.ExpectExec(
		regexp.QuoteMeta(`UPDATE "publishing_preview_tokens" SET "consumed_at"=$1 WHERE id = $2 AND token_hash = $3 AND consumed_at IS NULL AND created_at <= $4 AND expires_at > $5`),
	).WithArgs(
		now,
		"018f1f64-7b2a-7c10-8bf2-3c40d5f67585",
		hash[:], now, now,
	).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	err = repository.Consume(context.Background(), "018f1f64-7b2a-7c10-8bf2-3c40d5f67585", hash, now)
	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}

func previewRepositoryDatabase(t *testing.T) (*gormdb.DB, sqlmock.Sqlmock) {
	t.Helper()
	sqlDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })
	db, err := gormdb.Open(postgres.New(postgres.Config{Conn: sqlDB}), &gormdb.Config{DisableAutomaticPing: true})
	require.NoError(t, err)
	return db, mock
}
