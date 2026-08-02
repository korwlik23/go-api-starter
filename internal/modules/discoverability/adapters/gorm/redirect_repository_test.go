package gorm

import (
	"context"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	discoverabilitydomain "github.com/korwlik23/go-api-starter/internal/modules/discoverability/domain"
	"github.com/korwlik23/go-api-starter/internal/modules/discoverability/ports"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	gormdb "gorm.io/gorm"
)

func TestM3B5RedirectRepositoryListsAccountScopedRows(t *testing.T) {
	t.Parallel()
	db, mock := discoverabilityRepositoryDatabase(t)
	repository, err := NewRedirectRepository(db)
	require.NoError(t, err)
	now := time.Date(2026, 8, 2, 10, 0, 0, 0, time.UTC)
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT id, account_id, locale_id, source_path, destination_path, status_code, enabled, version, deleted_at, created_at, updated_at FROM "discoverability_redirects" WHERE account_id = $1 AND deleted_at IS NULL ORDER BY updated_at DESC, id DESC LIMIT $2`)).
		WithArgs("018f1f64-7b2a-7c10-8bf2-3c40d5f67580", 26).
		WillReturnRows(sqlmock.NewRows([]string{"id", "account_id", "locale_id", "source_path", "destination_path", "status_code", "enabled", "version", "deleted_at", "created_at", "updated_at"}).AddRow(
			"018f1f64-7b2a-7c10-8bf2-3c40d5f67581", "018f1f64-7b2a-7c10-8bf2-3c40d5f67580", "018f1f64-7b2a-7c10-8bf2-3c40d5f67582", "/old", "/new", int(discoverabilitydomain.StatusMovedPermanently), true, 1, nil, now, now,
		))
	page, err := repository.List(context.Background(), ports.RedirectListQuery{AccountID: "018f1f64-7b2a-7c10-8bf2-3c40d5f67580", Limit: 25})
	require.NoError(t, err)
	require.Len(t, page.Items, 1)
	require.Equal(t, "/new", page.Items[0].DestinationPath)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestM3B5SEODefaultsRepositoryMapsMissingRowsToNotFound(t *testing.T) {
	t.Parallel()
	db, mock := discoverabilityRepositoryDatabase(t)
	repository, err := NewRedirectRepository(db)
	require.NoError(t, err)
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT id, account_id, locale_id, title, description, canonical_base, robots, version, created_at, updated_at FROM "discoverability_seo_defaults" WHERE account_id = $1 AND locale_id = $2 LIMIT $3`)).
		WithArgs("018f1f64-7b2a-7c10-8bf2-3c40d5f67580", "018f1f64-7b2a-7c10-8bf2-3c40d5f67582", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id"}))
	_, err = repository.GetSEODefaults(context.Background(), "018f1f64-7b2a-7c10-8bf2-3c40d5f67580", "018f1f64-7b2a-7c10-8bf2-3c40d5f67582")
	require.ErrorIs(t, err, ports.ErrSEODefaultsNotFound)
}

func discoverabilityRepositoryDatabase(t *testing.T) (*gormdb.DB, sqlmock.Sqlmock) {
	t.Helper()
	sqlDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })
	db, err := gormdb.Open(postgres.New(postgres.Config{Conn: sqlDB}), &gormdb.Config{DisableAutomaticPing: true})
	require.NoError(t, err)
	return db, mock
}
