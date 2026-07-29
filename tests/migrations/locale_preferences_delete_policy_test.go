package migrations_test

import (
	"context"
	"database/sql"
	"fmt"
	"testing"

	"github.com/korwlik23/go-api-starter/internal/platform/config"
	"github.com/stretchr/testify/require"
)

func m16AssertPreferenceDeletePolicy(
	t *testing.T,
	ctx context.Context,
	db *sql.DB,
	cfg config.DatabaseConfig,
	seedOffset byte,
) {
	t.Helper()

	missingHeaderUser := m3UserID(t, cfg, 60+seedOffset)
	selectedUser := m3UserID(t, cfg, 61+seedOffset)
	otherUser := m3UserID(t, cfg, 62+seedOffset)
	headerDeleteUser := m3UserID(t, cfg, 63+seedOffset)
	allUser := m3UserID(t, cfg, 64+seedOffset)
	selectedLocale := m14LocaleID(t, cfg, 60+seedOffset)
	unselectedLocale := m14LocaleID(t, cfg, 61+seedOffset)
	headerDeleteLocale := m14LocaleID(t, cfg, 62+seedOffset)
	allFreeLocale := m14LocaleID(t, cfg, 63+seedOffset)

	for index, userID := range []any{
		missingHeaderUser,
		selectedUser,
		otherUser,
		headerDeleteUser,
		allUser,
	} {
		m16InsertUser(
			t,
			ctx,
			db,
			cfg,
			userID,
			fmt.Sprintf("m16-delete-%d-%d@example.test", seedOffset, index),
		)
	}
	for index, localeID := range []any{
		selectedLocale,
		unselectedLocale,
		headerDeleteLocale,
		allFreeLocale,
	} {
		m14InsertLocale(
			t,
			ctx,
			db,
			cfg,
			localeID,
			fmt.Sprintf("x-m16d%02x", byte(index)+seedOffset),
			fmt.Sprintf("M16 Delete %d", index),
			"ltr",
		)
	}

	_, err := db.ExecContext(
		ctx,
		m16PreferenceLocaleInsertSQL(cfg),
		missingHeaderUser,
		selectedLocale,
	)
	require.Error(t, err, "a selection must reference an existing preference header")
	require.Zero(t, m16PreferenceLocaleCount(t, ctx, db, cfg, missingHeaderUser, selectedLocale))

	m16InsertPreference(t, ctx, db, cfg, selectedUser, false)
	unknownLocale := m14LocaleID(t, cfg, 80+seedOffset)
	_, err = db.ExecContext(
		ctx,
		m16PreferenceLocaleInsertSQL(cfg),
		selectedUser,
		unknownLocale,
	)
	require.Error(t, err, "a selection must reference an existing locale")
	require.Zero(t, m16PreferenceLocaleCount(t, ctx, db, cfg, selectedUser, unknownLocale))

	m16InsertPreferenceLocale(t, ctx, db, cfg, selectedUser, selectedLocale)
	updatedLocaleID := m14LocaleID(t, cfg, 81+seedOffset)
	_, err = db.ExecContext(
		ctx,
		"UPDATE locales SET id = "+migrationPlaceholder(cfg, 1)+" WHERE id = "+migrationPlaceholder(cfg, 2),
		updatedLocaleID,
		selectedLocale,
	)
	require.Error(t, err, "selected locale identity must be immutable")
	require.Equal(t, 1, m16PreferenceLocaleCount(t, ctx, db, cfg, selectedUser, selectedLocale))

	_, err = db.ExecContext(
		ctx,
		"DELETE FROM locales WHERE id = "+migrationPlaceholder(cfg, 1),
		selectedLocale,
	)
	require.Error(t, err, "a selected locale must not be hard-deleted")
	require.True(t, m16LocaleExists(t, ctx, db, cfg, selectedLocale))

	updatedUserID := m3UserID(t, cfg, 81+seedOffset)
	_, err = db.ExecContext(
		ctx,
		"UPDATE users SET id = "+migrationPlaceholder(cfg, 1)+" WHERE id = "+migrationPlaceholder(cfg, 2),
		updatedUserID,
		selectedUser,
	)
	require.Error(t, err, "preference owner identity must be immutable")
	require.Equal(t, 1, m16PreferenceCount(t, ctx, db, cfg, selectedUser))

	_, err = db.ExecContext(
		ctx,
		"UPDATE locales SET enabled = "+migrationPlaceholder(cfg, 1)+
			", user_selectable = "+migrationPlaceholder(cfg, 2)+
			" WHERE id = "+migrationPlaceholder(cfg, 3),
		true,
		true,
		selectedLocale,
	)
	require.NoError(t, err)
	_, err = db.ExecContext(
		ctx,
		"UPDATE locales SET user_selectable = "+migrationPlaceholder(cfg, 1)+
			", enabled = "+migrationPlaceholder(cfg, 2)+
			" WHERE id = "+migrationPlaceholder(cfg, 3),
		false,
		false,
		selectedLocale,
	)
	require.NoError(t, err)
	require.Equal(t, 1, m16PreferenceLocaleCount(t, ctx, db, cfg, selectedUser, selectedLocale), "status drift must preserve dormant selection")

	_, err = db.ExecContext(
		ctx,
		"DELETE FROM locales WHERE id = "+migrationPlaceholder(cfg, 1),
		unselectedLocale,
	)
	require.NoError(t, err, "an unselected locale must remain independently deletable")
	require.False(t, m16LocaleExists(t, ctx, db, cfg, unselectedLocale))

	m16InsertPreference(t, ctx, db, cfg, otherUser, false)
	m16InsertPreferenceLocale(t, ctx, db, cfg, otherUser, selectedLocale)
	_, err = db.ExecContext(
		ctx,
		"DELETE FROM users WHERE id = "+migrationPlaceholder(cfg, 1),
		selectedUser,
	)
	require.NoError(t, err, "deleting a user must cascade only that user's preference")
	require.Zero(t, m16PreferenceCount(t, ctx, db, cfg, selectedUser))
	require.Zero(t, m16PreferenceLocaleCount(t, ctx, db, cfg, selectedUser, nil))
	require.Equal(t, 1, m16PreferenceCount(t, ctx, db, cfg, otherUser))
	require.Equal(t, 1, m16PreferenceLocaleCount(t, ctx, db, cfg, otherUser, selectedLocale))
	require.True(t, m16LocaleExists(t, ctx, db, cfg, selectedLocale))

	m16InsertPreference(t, ctx, db, cfg, headerDeleteUser, false)
	m16InsertPreferenceLocale(t, ctx, db, cfg, headerDeleteUser, headerDeleteLocale)
	_, err = db.ExecContext(
		ctx,
		"DELETE FROM user_locale_preferences WHERE user_id = "+migrationPlaceholder(cfg, 1),
		headerDeleteUser,
	)
	require.NoError(t, err, "explicit header teardown must cascade its selections")
	require.Zero(t, m16PreferenceLocaleCount(t, ctx, db, cfg, headerDeleteUser, nil))
	require.Equal(t, 1, m16UserCount(t, ctx, db, cfg, headerDeleteUser), "header teardown must not delete its user")

	m16InsertPreference(t, ctx, db, cfg, allUser, true)
	require.Zero(t, m16PreferenceLocaleCount(t, ctx, db, cfg, allUser, nil))
	_, err = db.ExecContext(
		ctx,
		"DELETE FROM locales WHERE id = "+migrationPlaceholder(cfg, 1),
		allFreeLocale,
	)
	require.NoError(t, err, "all-languages mode must not pin every locale")
	require.Equal(t, 1, m16PreferenceCount(t, ctx, db, cfg, allUser))

	_, err = db.ExecContext(
		ctx,
		"DELETE FROM user_locale_preference_locales WHERE user_id = "+migrationPlaceholder(cfg, 1)+
			" AND locale_id = "+migrationPlaceholder(cfg, 2),
		otherUser,
		selectedLocale,
	)
	require.NoError(t, err)
	_, err = db.ExecContext(
		ctx,
		"DELETE FROM locales WHERE id = "+migrationPlaceholder(cfg, 1),
		selectedLocale,
	)
	require.NoError(t, err, "locale hard-delete is possible only after explicit selection cleanup")
}

func m16LocaleExists(
	t *testing.T,
	ctx context.Context,
	db *sql.DB,
	cfg config.DatabaseConfig,
	localeID any,
) bool {
	t.Helper()
	var count int
	require.NoError(t, db.QueryRowContext(
		ctx,
		"SELECT COUNT(*) FROM locales WHERE id = "+migrationPlaceholder(cfg, 1),
		localeID,
	).Scan(&count))
	return count == 1
}

func m16UserCount(
	t *testing.T,
	ctx context.Context,
	db *sql.DB,
	cfg config.DatabaseConfig,
	userID any,
) int {
	t.Helper()
	var count int
	require.NoError(t, db.QueryRowContext(
		ctx,
		"SELECT COUNT(*) FROM users WHERE id = "+migrationPlaceholder(cfg, 1),
		userID,
	).Scan(&count))
	return count
}
