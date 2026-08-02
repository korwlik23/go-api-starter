package gorm

import (
	"context"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/korwlik23/go-api-starter/internal/modules/publishing/ports"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	gormdb "gorm.io/gorm"
)

func TestM3A6RevisionRepositoryListsAccountScopedProjection(t *testing.T) {
	t.Parallel()

	db, mock := revisionRepositoryDatabase(t)
	repository, err := NewRevisionRepository(db)
	require.NoError(t, err)
	createdAt := time.Date(2026, 8, 2, 12, 0, 0, 0, time.UTC)
	mock.ExpectQuery(regexp.QuoteMeta(
		`SELECT id AS revision_id, account_id, content_id, translation_id, author_id, snapshot_hash, change_summary, source_version, snapshot, created_at FROM "content_revisions" WHERE account_id = $1 AND content_id = $2 AND translation_id = $3 ORDER BY created_at DESC, id DESC LIMIT $4`,
	)).WithArgs(
		"018f1f64-7b2a-7c10-8bf2-3c40d5f67580",
		"018f1f64-7b2a-7c10-8bf2-3c40d5f67581",
		"018f1f64-7b2a-7c10-8bf2-3c40d5f67582",
		3,
	).WillReturnRows(sqlmock.NewRows([]string{
		"revision_id", "account_id", "content_id", "translation_id", "author_id",
		"snapshot_hash", "change_summary", "source_version", "snapshot", "created_at",
	}).AddRow(
		"018f1f64-7b2a-7c10-8bf2-3c40d5f67583",
		"018f1f64-7b2a-7c10-8bf2-3c40d5f67580",
		"018f1f64-7b2a-7c10-8bf2-3c40d5f67581",
		"018f1f64-7b2a-7c10-8bf2-3c40d5f67582",
		"018f1f64-7b2a-7c10-8bf2-3c40d5f67584",
		"0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
		"initial", 1,
		[]byte(`{"kind":"page","content_key":"home","locale_id":"018f1f64-7b2a-7c10-8bf2-3c40d5f67582","slug":"home","path":"/home","title":"Home","blocks":[],"seo":{},"geo":{},"aeo":{},"status":"draft"}`),
		createdAt,
	))

	page, err := repository.List(context.Background(), ports.RevisionListQuery{
		AccountID:     revisionAccountIDForAdapter,
		ContentID:     revisionContentIDForAdapter,
		TranslationID: revisionTranslationIDForAdapter,
		Limit:         2,
	})
	require.NoError(t, err)
	require.Len(t, page.Items, 1)
	require.Equal(t, int64(1), page.Items[0].Revision.SourceVersion)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestM3A6RevisionRepositoryMapsMissingRevisionToNotFound(t *testing.T) {
	t.Parallel()

	db, mock := revisionRepositoryDatabase(t)
	repository, err := NewRevisionRepository(db)
	require.NoError(t, err)
	mock.ExpectQuery(`SELECT .* FROM "content_revisions".*WHERE account_id = \$1 AND content_id = \$2 AND id = \$3`).
		WithArgs(
			revisionAccountIDForAdapter, revisionContentIDForAdapter, revisionIDForAdapter,
			1,
		).
		WillReturnRows(sqlmock.NewRows([]string{"revision_id"}))

	_, err = repository.Get(context.Background(), revisionAccountIDForAdapter, revisionContentIDForAdapter, revisionIDForAdapter)
	require.ErrorIs(t, err, ports.ErrRevisionNotFound)
	require.NoError(t, mock.ExpectationsWereMet())
}

const (
	revisionAccountIDForAdapter     = "018f1f64-7b2a-7c10-8bf2-3c40d5f67580"
	revisionContentIDForAdapter     = "018f1f64-7b2a-7c10-8bf2-3c40d5f67581"
	revisionTranslationIDForAdapter = "018f1f64-7b2a-7c10-8bf2-3c40d5f67582"
	revisionIDForAdapter            = "018f1f64-7b2a-7c10-8bf2-3c40d5f67583"
)

func revisionRepositoryDatabase(t *testing.T) (*gormdb.DB, sqlmock.Sqlmock) {
	t.Helper()
	sqlDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })
	db, err := gormdb.Open(postgres.New(postgres.Config{Conn: sqlDB}), &gormdb.Config{DisableAutomaticPing: true})
	require.NoError(t, err)
	return db, mock
}
