package gorm

import (
	"context"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	gormdb "gorm.io/gorm"
)

func TestLocaleRepositoryListsEnabledRegistryDeterministically(t *testing.T) {
	db, mock := localeRepositoryDatabase(t)
	repository, err := NewLocaleRepository(db)
	require.NoError(t, err)
	defaultSlot := int16(1)
	mock.ExpectQuery(`SELECT .* FROM "locales".*ORDER BY tag ASC`).
		WithArgs(true).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "tag", "name", "direction", "fallback_locale_id",
			"enabled", "user_selectable", "default_slot", "version",
		}).AddRow(
			"018f1f64-7b2a-7c10-8bf2-3c40d5f67581",
			"en",
			"English",
			"ltr",
			nil,
			true,
			true,
			defaultSlot,
			3,
		).AddRow(
			"018f1f64-7b2a-7c10-8bf2-3c40d5f67582",
			"th",
			"ไทย",
			"ltr",
			"018f1f64-7b2a-7c10-8bf2-3c40d5f67581",
			true,
			false,
			nil,
			2,
		))

	locales, err := repository.ListEnabled(context.Background())

	require.NoError(t, err)
	require.Len(t, locales, 2)
	require.True(t, locales[0].Default)
	require.False(t, locales[1].UserSelectable)
	require.Equal(t, locales[0].ID, locales[1].FallbackID)
	require.NoError(t, mock.ExpectationsWereMet())
}

func localeRepositoryDatabase(t *testing.T) (*gormdb.DB, sqlmock.Sqlmock) {
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
