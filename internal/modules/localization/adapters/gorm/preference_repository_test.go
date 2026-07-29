package gorm

import (
	"context"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
)

func TestPreferenceRepositoryReadsOnlyCurrentlySelectableSubset(
	t *testing.T,
) {
	db, mock := localeRepositoryDatabase(t)
	repository, err := NewPreferenceRepository(db, time.Now)
	require.NoError(t, err)
	userID := "018f1f64-7b2a-7c10-8bf2-3c40d5f67581"
	mock.ExpectQuery(`SELECT .* FROM "user_locale_preferences"`).
		WithArgs(userID, 1).
		WillReturnRows(sqlmock.NewRows([]string{
			"user_id", "all_languages", "version",
		}).AddRow(userID, false, 4))
	mock.ExpectQuery(
		`SELECT locales.id AS locale_id FROM "user_locale_preference_locales"`,
	).
		WithArgs(userID, true, true).
		WillReturnRows(sqlmock.NewRows([]string{"locale_id"}).
			AddRow("018f1f64-7b2a-7c10-8bf2-3c40d5f67582").
			AddRow("018f1f64-7b2a-7c10-8bf2-3c40d5f67583"))

	preference, err := repository.Load(context.Background(), userID)

	require.NoError(t, err)
	require.False(t, preference.AllLanguages)
	require.Equal(t, int64(4), preference.Version)
	require.Len(t, preference.LocaleIDs, 2)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestPreferenceRepositoryReturnsSystemDefaultWhenSubsetBecomesEmpty(
	t *testing.T,
) {
	db, mock := localeRepositoryDatabase(t)
	repository, err := NewPreferenceRepository(db, time.Now)
	require.NoError(t, err)
	userID := "018f1f64-7b2a-7c10-8bf2-3c40d5f67581"
	defaultID := "018f1f64-7b2a-7c10-8bf2-3c40d5f67582"
	mock.ExpectQuery(`SELECT .* FROM "user_locale_preferences"`).
		WillReturnRows(sqlmock.NewRows([]string{
			"user_id", "all_languages", "version",
		}).AddRow(userID, false, 4))
	mock.ExpectQuery(
		`SELECT locales.id AS locale_id FROM "user_locale_preference_locales"`,
	).WillReturnRows(sqlmock.NewRows([]string{"locale_id"}))
	mock.ExpectQuery(`SELECT .* FROM "locales"`).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "tag", "name", "direction", "enabled", "user_selectable",
			"default_slot", "version",
		}).AddRow(
			defaultID, "en", "English", "ltr", true, true, 1, 2,
		))

	preference, err := repository.Load(context.Background(), userID)

	require.NoError(t, err)
	require.False(t, preference.AllLanguages)
	require.Equal(t, []string{defaultID}, preference.LocaleIDs)
}
