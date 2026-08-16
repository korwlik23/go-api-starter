package gorm

import (
	"context"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
	mysqlgorm "gorm.io/driver/mysql"
	gormdb "gorm.io/gorm"
)

func TestRepositoryListQuotesReservedKeyForMariaDB(t *testing.T) {
	db, mock := settingsMySQLDatabase(t)
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	mock.ExpectQuery(regexp.QuoteMeta("SELECT * FROM `system_settings` ORDER BY `key` ASC")).
		WillReturnRows(sqlmock.NewRows([]string{"key", "value", "version", "created_at", "updated_at"}).AddRow(
			"site.name", "Go Starter", int64(1), now, now,
		))

	repository, err := NewRepository(db)
	require.NoError(t, err)
	items, err := repository.List(context.Background())
	require.NoError(t, err)
	require.Len(t, items, 1)
	require.Equal(t, "site.name", items[0].Key)
	require.NoError(t, mock.ExpectationsWereMet())
}

func settingsMySQLDatabase(t *testing.T) (*gormdb.DB, sqlmock.Sqlmock) {
	t.Helper()
	sqlDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })
	db, err := gormdb.Open(
		mysqlgorm.New(mysqlgorm.Config{Conn: sqlDB, SkipInitializeWithVersion: true}),
		&gormdb.Config{DisableAutomaticPing: true},
	)
	require.NoError(t, err)
	return db, mock
}
