package gorm

import (
	"context"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	navigationdomain "github.com/korwlik23/go-api-starter/internal/modules/navigation/domain"
	"github.com/korwlik23/go-api-starter/internal/modules/navigation/ports"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	gormdb "gorm.io/gorm"
)

func TestM3B4MenuRepositoryListsBoundedAccountProjection(t *testing.T) {
	t.Parallel()

	db, mock := navigationRepositoryDatabase(t)
	repository, err := NewMenuRepository(db)
	require.NoError(t, err)
	updatedAt := time.Date(2026, 8, 2, 10, 0, 0, 0, time.UTC)
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT id, account_id, locale_id, menu_key, name, enabled, version, deleted_at, created_at, updated_at FROM "navigation_menus" WHERE account_id = $1 AND deleted_at IS NULL ORDER BY updated_at DESC, id DESC LIMIT $2`)).
		WithArgs("018f1f64-7b2a-7c10-8bf2-3c40d5f67580", 26).
		WillReturnRows(sqlmock.NewRows([]string{"id", "account_id", "locale_id", "menu_key", "name", "enabled", "version", "deleted_at", "created_at", "updated_at"}).AddRow(
			"018f1f64-7b2a-7c10-8bf2-3c40d5f67581", "018f1f64-7b2a-7c10-8bf2-3c40d5f67580", "018f1f64-7b2a-7c10-8bf2-3c40d5f67582", "main", "Main", true, 2, nil, updatedAt, updatedAt,
		))
	page, err := repository.List(context.Background(), ports.MenuListQuery{AccountID: "018f1f64-7b2a-7c10-8bf2-3c40d5f67580", Limit: 25})
	require.NoError(t, err)
	require.Len(t, page.Items, 1)
	require.Equal(t, "main", page.Items[0].Key)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestM3B4PublicMenuResolverScopesAccountLocaleAndEnabledItems(t *testing.T) {
	t.Parallel()

	db, mock := navigationRepositoryDatabase(t)
	repository, err := NewMenuRepository(db)
	require.NoError(t, err)
	updatedAt := time.Date(2026, 8, 2, 10, 0, 0, 0, time.UTC)
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT id, account_id, locale_id, menu_key, name, enabled, version, deleted_at, created_at, updated_at FROM "navigation_menus" WHERE account_id = $1 AND locale_id = $2 AND menu_key = $3 AND enabled = $4 AND deleted_at IS NULL LIMIT $5`)).
		WithArgs("018f1f64-7b2a-7c10-8bf2-3c40d5f67580", "018f1f64-7b2a-7c10-8bf2-3c40d5f67582", "main", true, 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "account_id", "locale_id", "menu_key", "name", "enabled", "version", "deleted_at", "created_at", "updated_at"}).AddRow(
			"018f1f64-7b2a-7c10-8bf2-3c40d5f67581", "018f1f64-7b2a-7c10-8bf2-3c40d5f67580", "018f1f64-7b2a-7c10-8bf2-3c40d5f67582", "main", "Main", true, 2, nil, updatedAt, updatedAt,
		))
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT id, account_id, menu_id, parent_id, label, target_kind, target, position, enabled, version, deleted_at, created_at, updated_at FROM "navigation_menu_items" WHERE (account_id = $1 AND menu_id = $2 AND deleted_at IS NULL) AND enabled = $3 ORDER BY position ASC, id ASC`)).
		WithArgs("018f1f64-7b2a-7c10-8bf2-3c40d5f67580", "018f1f64-7b2a-7c10-8bf2-3c40d5f67581", true).
		WillReturnRows(sqlmock.NewRows([]string{"id", "account_id", "menu_id", "parent_id", "label", "target_kind", "target", "position", "enabled", "version", "deleted_at", "created_at", "updated_at"}).AddRow(
			"018f1f64-7b2a-7c10-8bf2-3c40d5f67583", "018f1f64-7b2a-7c10-8bf2-3c40d5f67580", "018f1f64-7b2a-7c10-8bf2-3c40d5f67581", nil, "Home", string(navigationdomain.TargetInternalPath), "/home", 0, true, 1, nil, updatedAt, updatedAt,
		))
	menu, err := repository.FindPublic(context.Background(), "018f1f64-7b2a-7c10-8bf2-3c40d5f67580", "018f1f64-7b2a-7c10-8bf2-3c40d5f67582", "main")
	require.NoError(t, err)
	require.Len(t, menu.Items, 1)
	require.Equal(t, "/home", menu.Items[0].Target)
	require.NoError(t, mock.ExpectationsWereMet())
}

func navigationRepositoryDatabase(t *testing.T) (*gormdb.DB, sqlmock.Sqlmock) {
	t.Helper()
	sqlDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })
	db, err := gormdb.Open(postgres.New(postgres.Config{Conn: sqlDB}), &gormdb.Config{DisableAutomaticPing: true})
	require.NoError(t, err)
	return db, mock
}
