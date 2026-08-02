package gorm

import (
	"context"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	publishingdomain "github.com/korwlik23/go-api-starter/internal/modules/publishing/domain"
	"github.com/korwlik23/go-api-starter/internal/modules/publishing/ports"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	gormdb "gorm.io/gorm"
)

func TestM3A4ContentRepositoryListsBoundedAccountProjection(t *testing.T) {
	t.Parallel()

	db, mock := contentRepositoryDatabase(t)
	repository, err := NewContentRepository(db)
	require.NoError(t, err)
	updatedAt := time.Date(2026, 8, 2, 10, 0, 0, 0, time.UTC)
	mock.ExpectQuery(regexp.QuoteMeta(
		`SELECT content_items.id AS content_id, content_items.account_id, content_items.kind, content_items.content_key, content_items.status AS content_status, content_items.version AS content_version, content_translations.id AS translation_id, content_translations.locale_id, content_translations.slug, content_translations.path, content_translations.title, content_translations.status AS translation_status, content_translations.version AS translation_version, content_translations.updated_at FROM "content_translations" JOIN content_items ON content_items.id = content_translations.content_id AND content_items.account_id = content_translations.account_id WHERE content_items.account_id = $1 AND content_translations.locale_id = $2 AND content_items.deleted_at IS NULL AND content_translations.deleted_at IS NULL ORDER BY content_translations.updated_at DESC, content_translations.id DESC LIMIT $3`,
	)).WithArgs(
		"018f1f64-7b2a-7c10-8bf2-3c40d5f67585",
		"018f1f64-7b2a-7c10-8bf2-3c40d5f67586",
		26,
	).WillReturnRows(sqlmock.NewRows([]string{
		"content_id", "account_id", "kind", "content_key", "content_status",
		"content_version", "translation_id", "locale_id", "slug", "path",
		"title", "translation_status", "translation_version", "updated_at",
	}).AddRow(
		"018f1f64-7b2a-7c10-8bf2-3c40d5f67587",
		"018f1f64-7b2a-7c10-8bf2-3c40d5f67585",
		"page", "home", "draft", 2,
		"018f1f64-7b2a-7c10-8bf2-3c40d5f67588",
		"018f1f64-7b2a-7c10-8bf2-3c40d5f67586",
		"home", "/home", "Home", "draft", 3, updatedAt,
	))

	page, err := repository.List(context.Background(), ports.ContentListQuery{
		AccountID: "018f1f64-7b2a-7c10-8bf2-3c40d5f67585",
		LocaleID:  "018f1f64-7b2a-7c10-8bf2-3c40d5f67586",
		Limit:     25,
	})
	require.NoError(t, err)
	require.Len(t, page.Items, 1)
	require.Nil(t, page.Next)
	require.Equal(t, "home", page.Items[0].ContentKey)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestM3A4PublicResolverExcludesDraftsAndScopesAccount(t *testing.T) {
	t.Parallel()

	db, mock := contentRepositoryDatabase(t)
	repository, err := NewContentRepository(db)
	require.NoError(t, err)
	mock.ExpectQuery(
		regexp.QuoteMeta("SELECT content_items.id AS content_id")+
			`.*`+
			regexp.QuoteMeta("WHERE content_items.account_id = $1")+
			`.*`+
			regexp.QuoteMeta("content_items.status = $4")+
			`.*`+
			regexp.QuoteMeta("content_translations.status = $5"),
	).
		WithArgs(
			"018f1f64-7b2a-7c10-8bf2-3c40d5f67585",
			"018f1f64-7b2a-7c10-8bf2-3c40d5f67586",
			"/home",
			string(publishingdomain.StatusPublished),
			string(publishingdomain.StatusPublished),
			1,
		).
		WillReturnRows(sqlmock.NewRows([]string{
			"content_id", "translation_id", "account_id", "locale_id", "kind",
			"content_key", "content_status", "content_version", "translation_status",
			"translation_version", "slug", "path", "title", "excerpt", "blocks",
			"updated_at",
		}).AddRow(
			"018f1f64-7b2a-7c10-8bf2-3c40d5f67587",
			"018f1f64-7b2a-7c10-8bf2-3c40d5f67588",
			"018f1f64-7b2a-7c10-8bf2-3c40d5f67585",
			"018f1f64-7b2a-7c10-8bf2-3c40d5f67586",
			"page", "home", "published", 2, "published", 3,
			"home", "/home", "Home", "", []byte(`[{"type":"text","data":{"text":"Home"}}]`),
			time.Now().UTC(),
		))

	detail, err := repository.FindPublic(
		context.Background(),
		"018f1f64-7b2a-7c10-8bf2-3c40d5f67585",
		"018f1f64-7b2a-7c10-8bf2-3c40d5f67586",
		"/home",
	)
	require.NoError(t, err)
	require.Equal(t, publishingdomain.StatusPublished, detail.TranslationStatus)
	require.Len(t, detail.Blocks, 1)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestM3A4ContentRepositoryMapsNoRowsToNotFound(t *testing.T) {
	t.Parallel()

	db, mock := contentRepositoryDatabase(t)
	repository, err := NewContentRepository(db)
	require.NoError(t, err)
	mock.ExpectQuery(
		regexp.QuoteMeta("SELECT content_items.id AS content_id")+
			`.*`+
			regexp.QuoteMeta("WHERE content_items.account_id = $1 AND content_translations.content_id = $2"),
	).
		WithArgs(
			"018f1f64-7b2a-7c10-8bf2-3c40d5f67585",
			"018f1f64-7b2a-7c10-8bf2-3c40d5f67587",
			"018f1f64-7b2a-7c10-8bf2-3c40d5f67586",
			1,
		).
		WillReturnRows(sqlmock.NewRows([]string{"content_id"}))

	_, err = repository.Get(
		context.Background(),
		"018f1f64-7b2a-7c10-8bf2-3c40d5f67585",
		"018f1f64-7b2a-7c10-8bf2-3c40d5f67587",
		"018f1f64-7b2a-7c10-8bf2-3c40d5f67586",
	)
	require.ErrorIs(t, err, ports.ErrContentNotFound)
}

func TestM3C1ContentRepositoryListsOnlyPublishedAlternates(t *testing.T) {
	t.Parallel()

	db, mock := contentRepositoryDatabase(t)
	repository, err := NewContentRepository(db)
	require.NoError(t, err)
	updatedAt := time.Date(2026, 8, 2, 11, 0, 0, 0, time.UTC)
	mock.ExpectQuery(
		regexp.QuoteMeta("SELECT content_items.id AS content_id")+
			`.*`+
			regexp.QuoteMeta("content_translations.locale_id <> $3")+
			`.*`+
			regexp.QuoteMeta("content_items.status = $4 AND content_translations.status = $5"),
	).
		WithArgs(
			"018f1f64-7b2a-7c10-8bf2-3c40d5f67580",
			"018f1f64-7b2a-7c10-8bf2-3c40d5f67581",
			"018f1f64-7b2a-7c10-8bf2-3c40d5f67583",
			string(publishingdomain.StatusPublished), string(publishingdomain.StatusPublished),
		).
		WillReturnRows(sqlmock.NewRows([]string{
			"content_id", "translation_id", "account_id", "locale_id", "kind",
			"content_key", "content_status", "content_version", "translation_status",
			"translation_version", "slug", "path", "title", "excerpt", "blocks",
			"updated_at",
		}).AddRow(
			"018f1f64-7b2a-7c10-8bf2-3c40d5f67581",
			"018f1f64-7b2a-7c10-8bf2-3c40d5f67584",
			"018f1f64-7b2a-7c10-8bf2-3c40d5f67580",
			"018f1f64-7b2a-7c10-8bf2-3c40d5f67584",
			"page", "home", "published", 2, "published", 3,
			"home", "/home", "Home", "", []byte(`[{"type":"text","data":{"text":"Home"}}]`), updatedAt,
		))

	items, err := repository.ListPublishedAlternates(
		context.Background(),
		"018f1f64-7b2a-7c10-8bf2-3c40d5f67580",
		"018f1f64-7b2a-7c10-8bf2-3c40d5f67581",
		"018f1f64-7b2a-7c10-8bf2-3c40d5f67583",
	)
	require.NoError(t, err)
	require.Len(t, items, 1)
	require.Equal(t, publishingdomain.StatusPublished, items[0].TranslationStatus)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestM3C1ContentRepositoryListsPublishedPostsWithPublicationPredicates(t *testing.T) {
	t.Parallel()

	db, mock := contentRepositoryDatabase(t)
	repository, err := NewContentRepository(db)
	require.NoError(t, err)
	updatedAt := time.Date(2026, 8, 2, 11, 0, 0, 0, time.UTC)
	mock.ExpectQuery(
		regexp.QuoteMeta("SELECT content_items.id AS content_id")+
			`.*`+
			regexp.QuoteMeta("content_items.kind = $3 AND content_items.status = $4")+
			`.*`+
			regexp.QuoteMeta("content_translations.status = $5"),
	).
		WithArgs(
			"018f1f64-7b2a-7c10-8bf2-3c40d5f67580",
			"018f1f64-7b2a-7c10-8bf2-3c40d5f67583",
			string(publishingdomain.KindPost), string(publishingdomain.StatusPublished),
			string(publishingdomain.StatusPublished), 3,
		).
		WillReturnRows(sqlmock.NewRows([]string{
			"content_id", "account_id", "kind", "content_key", "content_status",
			"content_version", "translation_id", "locale_id", "slug", "path",
			"title", "translation_status", "translation_version", "updated_at",
		}).AddRow(
			"018f1f64-7b2a-7c10-8bf2-3c40d5f67581",
			"018f1f64-7b2a-7c10-8bf2-3c40d5f67580",
			"post", "hello", "published", 2,
			"018f1f64-7b2a-7c10-8bf2-3c40d5f67582",
			"018f1f64-7b2a-7c10-8bf2-3c40d5f67583",
			"hello", "/news/hello", "Hello", "published", 3, updatedAt,
		))

	page, err := repository.ListPublishedPosts(
		context.Background(),
		"018f1f64-7b2a-7c10-8bf2-3c40d5f67580",
		"018f1f64-7b2a-7c10-8bf2-3c40d5f67583",
		2,
		nil,
	)
	require.NoError(t, err)
	require.Len(t, page.Items, 1)
	require.Equal(t, publishingdomain.KindPost, page.Items[0].Kind)
	require.Equal(t, publishingdomain.StatusPublished, page.Items[0].ContentStatus)
	require.NoError(t, mock.ExpectationsWereMet())
}

func contentRepositoryDatabase(t *testing.T) (*gormdb.DB, sqlmock.Sqlmock) {
	t.Helper()
	sqlDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })
	db, err := gormdb.Open(
		postgres.New(postgres.Config{Conn: sqlDB}),
		&gormdb.Config{DisableAutomaticPing: true},
	)
	require.NoError(t, err)
	return db, mock
}
