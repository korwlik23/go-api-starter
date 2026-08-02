package gorm

import (
	"context"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	mediadomain "github.com/korwlik23/go-api-starter/internal/modules/media/domain"
	mediaports "github.com/korwlik23/go-api-starter/internal/modules/media/ports"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	gormdb "gorm.io/gorm"
)

func TestM3B2AssetRepositoryRejectsNilDatabase(t *testing.T) {
	t.Parallel()

	_, err := NewAssetRepository(nil)
	require.ErrorIs(t, err, ErrInvalidAssetRepository)
}

func TestM3B2AssetRepositoryGetsUploadWithAccountPredicate(t *testing.T) {
	t.Parallel()

	db, mock := mediaRepositoryDatabase(t)
	repository, err := NewAssetRepository(db)
	require.NoError(t, err)
	now := time.Date(2026, 8, 2, 11, 0, 0, 0, time.UTC)
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT id, account_id, storage_key, original_name, mime_type, declared_size, expected_sha256, state, asset_id, version, expires_at, created_at, updated_at FROM "media_uploads" WHERE account_id = $1 AND id = $2 LIMIT $3`)).WithArgs(
		"018f1f64-7b2a-7c10-8bf2-3c40d5f67586", "018f1f64-7b2a-7c10-8bf2-3c40d5f67585", 1,
	).WillReturnRows(sqlmock.NewRows([]string{"id", "account_id", "storage_key", "original_name", "mime_type", "declared_size", "expected_sha256", "state", "asset_id", "version", "expires_at", "created_at", "updated_at"}).AddRow(
		"018f1f64-7b2a-7c10-8bf2-3c40d5f67585", "018f1f64-7b2a-7c10-8bf2-3c40d5f67586", "018f1f647b2a7c108bf23c40d5f67585.blob", "hero.png", "image/png", 8, "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef", "pending", nil, 1, now.Add(time.Hour), now, now,
	))
	upload, err := repository.GetUpload(context.Background(), "018f1f64-7b2a-7c10-8bf2-3c40d5f67586", "018f1f64-7b2a-7c10-8bf2-3c40d5f67585")
	require.NoError(t, err)
	require.Equal(t, mediadomain.UploadPending, upload.State)
	require.Equal(t, "hero.png", upload.OriginalName)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestM3B2AssetRepositoryListExcludesSoftDeletedRows(t *testing.T) {
	t.Parallel()

	db, mock := mediaRepositoryDatabase(t)
	repository, err := NewAssetRepository(db)
	require.NoError(t, err)
	now := time.Date(2026, 8, 2, 11, 0, 0, 0, time.UTC)
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT id, account_id, storage_key, original_name, mime_type, byte_size, sha256, width, height, version, deleted_at, created_at, updated_at FROM "media_assets" WHERE account_id = $1 AND deleted_at IS NULL ORDER BY updated_at DESC, id DESC LIMIT $2`)).WithArgs(
		"018f1f64-7b2a-7c10-8bf2-3c40d5f67586", 26,
	).WillReturnRows(sqlmock.NewRows([]string{"id", "account_id", "storage_key", "original_name", "mime_type", "byte_size", "sha256", "width", "height", "version", "deleted_at", "created_at", "updated_at"}).AddRow(
		"018f1f64-7b2a-7c10-8bf2-3c40d5f67590", "018f1f64-7b2a-7c10-8bf2-3c40d5f67586", "018f1f647b2a7c108bf23c40d5f67585.blob", "hero.png", "image/png", 8, "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef", nil, nil, 1, nil, now, now,
	))
	page, err := repository.ListAssets(context.Background(), mediaports.AssetListQuery{AccountID: "018f1f64-7b2a-7c10-8bf2-3c40d5f67586", Limit: 25})
	require.NoError(t, err)
	require.Len(t, page.Items, 1)
	require.NoError(t, mock.ExpectationsWereMet())
}

func mediaRepositoryDatabase(t *testing.T) (*gormdb.DB, sqlmock.Sqlmock) {
	t.Helper()
	sqlDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })
	db, err := gormdb.Open(postgres.New(postgres.Config{Conn: sqlDB}), &gormdb.Config{DisableAutomaticPing: true})
	require.NoError(t, err)
	return db, mock
}
