package migrations_test

import (
	"context"
	"database/sql"
	"fmt"
	"testing"
	"time"

	"github.com/korwlik23/go-api-starter/internal/platform/config"
	"github.com/stretchr/testify/require"
)

type m16PreferenceRecord struct {
	allLanguages bool
	version      int64
	createdAt    time.Time
	updatedAt    time.Time
}

func m16AssertPreferenceBehavior(
	t *testing.T,
	ctx context.Context,
	db *sql.DB,
	cfg config.DatabaseConfig,
	seedOffset byte,
) {
	t.Helper()

	allUser := m3UserID(t, cfg, 10+seedOffset)
	subsetUser := m3UserID(t, cfg, 11+seedOffset)
	otherUser := m3UserID(t, cfg, 12+seedOffset)
	missingUser := m3UserID(t, cfg, 13+seedOffset)
	firstLocale := m14LocaleID(t, cfg, 10+seedOffset)
	secondLocale := m14LocaleID(t, cfg, 11+seedOffset)
	laterLocale := m14LocaleID(t, cfg, 12+seedOffset)

	for index, userID := range []any{allUser, subsetUser, otherUser, missingUser} {
		m16InsertUser(
			t,
			ctx,
			db,
			cfg,
			userID,
			fmt.Sprintf("m16-behavior-%d-%d@example.test", seedOffset, index),
		)
	}
	for index, localeID := range []any{firstLocale, secondLocale} {
		m14InsertLocale(
			t,
			ctx,
			db,
			cfg,
			localeID,
			fmt.Sprintf("x-m16b%02x", byte(index)+seedOffset),
			fmt.Sprintf("M16 Behavior %d", index),
			"ltr",
		)
	}

	require.Zero(t, m16PreferenceCount(t, ctx, db, cfg, missingUser))
	m16InsertPreference(t, ctx, db, cfg, allUser, true)
	allRecord := m16ReadPreference(t, ctx, db, cfg, allUser)
	require.True(t, allRecord.allLanguages)
	require.Equal(t, int64(1), allRecord.version)
	require.False(t, allRecord.createdAt.IsZero())
	require.False(t, allRecord.updatedAt.IsZero())
	require.False(t, allRecord.updatedAt.Before(allRecord.createdAt))
	require.Zero(t, m16PreferenceLocaleCount(t, ctx, db, cfg, allUser, nil))

	m14InsertLocale(
		t,
		ctx,
		db,
		cfg,
		laterLocale,
		fmt.Sprintf("x-m16l%02x", seedOffset),
		"M16 Later Locale",
		"ltr",
	)
	require.Zero(t, m16PreferenceLocaleCount(t, ctx, db, cfg, allUser, nil), "all mode must not materialize locale rows")

	m16InsertPreference(t, ctx, db, cfg, subsetUser, false)
	m16InsertPreferenceLocale(t, ctx, db, cfg, subsetUser, firstLocale)
	m16InsertPreferenceLocale(t, ctx, db, cfg, subsetUser, secondLocale)
	subsetRecord := m16ReadPreference(t, ctx, db, cfg, subsetUser)
	require.False(t, subsetRecord.allLanguages)
	require.Equal(t, int64(1), subsetRecord.version)
	require.Equal(t, 2, m16PreferenceLocaleCount(t, ctx, db, cfg, subsetUser, nil))

	m16InsertPreference(t, ctx, db, cfg, otherUser, false)
	m16InsertPreferenceLocale(t, ctx, db, cfg, otherUser, firstLocale)
	require.Equal(t, 1, m16PreferenceLocaleCount(t, ctx, db, cfg, otherUser, firstLocale))
	require.Equal(t, 1, m16PreferenceLocaleCount(t, ctx, db, cfg, subsetUser, firstLocale))

	_, err := db.ExecContext(
		ctx,
		m16PreferenceInsertSQL(cfg),
		subsetUser,
		true,
	)
	require.Error(t, err, "one user must have only one preference header")
	require.False(t, m16ReadPreference(t, ctx, db, cfg, subsetUser).allLanguages)

	_, err = db.ExecContext(
		ctx,
		m16PreferenceLocaleInsertSQL(cfg),
		subsetUser,
		firstLocale,
	)
	require.Error(t, err, "an exact user-locale selection must be unique")
	require.Equal(t, 1, m16PreferenceLocaleCount(t, ctx, db, cfg, subsetUser, firstLocale))
}

func m16InsertUser(
	t *testing.T,
	ctx context.Context,
	db *sql.DB,
	cfg config.DatabaseConfig,
	userID any,
	email string,
) {
	t.Helper()
	_, err := db.ExecContext(
		ctx,
		"INSERT INTO users (id, normalized_email, status) VALUES ("+migrationPlaceholders(cfg, 3)+")",
		userID,
		email,
		"active",
	)
	require.NoError(t, err)
}

func m16InsertPreference(
	t *testing.T,
	ctx context.Context,
	db *sql.DB,
	cfg config.DatabaseConfig,
	userID any,
	allLanguages bool,
) {
	t.Helper()
	_, err := db.ExecContext(ctx, m16PreferenceInsertSQL(cfg), userID, allLanguages)
	require.NoError(t, err)
}

func m16InsertPreferenceLocale(
	t *testing.T,
	ctx context.Context,
	db *sql.DB,
	cfg config.DatabaseConfig,
	userID any,
	localeID any,
) {
	t.Helper()
	_, err := db.ExecContext(ctx, m16PreferenceLocaleInsertSQL(cfg), userID, localeID)
	require.NoError(t, err)
}

func m16PreferenceInsertSQL(cfg config.DatabaseConfig) string {
	return "INSERT INTO user_locale_preferences (user_id, all_languages) VALUES (" +
		migrationPlaceholders(cfg, 2) + ")"
}

func m16PreferenceLocaleInsertSQL(cfg config.DatabaseConfig) string {
	return "INSERT INTO user_locale_preference_locales (user_id, locale_id) VALUES (" +
		migrationPlaceholders(cfg, 2) + ")"
}

func m16ReadPreference(
	t *testing.T,
	ctx context.Context,
	db *sql.DB,
	cfg config.DatabaseConfig,
	userID any,
) m16PreferenceRecord {
	t.Helper()
	var record m16PreferenceRecord
	require.NoError(t, db.QueryRowContext(
		ctx,
		"SELECT all_languages, version, created_at, updated_at FROM user_locale_preferences WHERE user_id = "+
			migrationPlaceholder(cfg, 1),
		userID,
	).Scan(&record.allLanguages, &record.version, &record.createdAt, &record.updatedAt))
	return record
}

func m16PreferenceCount(
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
		"SELECT COUNT(*) FROM user_locale_preferences WHERE user_id = "+migrationPlaceholder(cfg, 1),
		userID,
	).Scan(&count))
	return count
}

func m16PreferenceLocaleCount(
	t *testing.T,
	ctx context.Context,
	db *sql.DB,
	cfg config.DatabaseConfig,
	userID any,
	localeID any,
) int {
	t.Helper()
	query := "SELECT COUNT(*) FROM user_locale_preference_locales WHERE user_id = " +
		migrationPlaceholder(cfg, 1)
	arguments := []any{userID}
	if localeID != nil {
		query += " AND locale_id = " + migrationPlaceholder(cfg, 2)
		arguments = append(arguments, localeID)
	}
	var count int
	require.NoError(t, db.QueryRowContext(ctx, query, arguments...).Scan(&count))
	return count
}
