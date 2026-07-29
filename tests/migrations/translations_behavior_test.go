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

type m15Fixtures struct {
	english any
	thai    any
	arabic  any
	suffix  string
}

type m15TranslationRecord struct {
	value     sql.NullString
	source    string
	version   int64
	createdAt time.Time
	updatedAt time.Time
}

func m15AssertTranslationBehavior(
	t *testing.T,
	ctx context.Context,
	db *sql.DB,
	cfg config.DatabaseConfig,
	seedOffset byte,
) {
	t.Helper()

	fixtures := m15CreateLocaleFixtures(t, ctx, db, cfg, seedOffset)
	m15AssertTranslationIdentityAndRoundTrip(t, ctx, db, cfg, fixtures)
	m15AssertTranslationConstraints(t, ctx, db, cfg, fixtures)
	m15AssertTranslationVersionLifecycle(t, ctx, db, cfg, fixtures)
	m15AssertTranslationDeletePolicy(t, ctx, db, cfg, fixtures, seedOffset)
}

func m15CreateLocaleFixtures(
	t *testing.T,
	ctx context.Context,
	db *sql.DB,
	cfg config.DatabaseConfig,
	seedOffset byte,
) m15Fixtures {
	t.Helper()

	suffix := "a"
	if seedOffset != 0 {
		suffix = "b"
	}
	fixtures := m15Fixtures{
		english: m14LocaleID(t, cfg, 130+seedOffset),
		thai:    m14LocaleID(t, cfg, 131+seedOffset),
		arabic:  m14LocaleID(t, cfg, 132+seedOffset),
		suffix:  suffix,
	}
	m14InsertLocale(
		t,
		ctx,
		db,
		cfg,
		fixtures.english,
		"en-x-m15"+suffix,
		"English M15 "+suffix,
		"ltr",
	)
	m14InsertLocale(
		t,
		ctx,
		db,
		cfg,
		fixtures.thai,
		"th-x-m15"+suffix,
		"ไทย M15 "+suffix,
		"ltr",
	)
	m14InsertLocale(
		t,
		ctx,
		db,
		cfg,
		fixtures.arabic,
		"ar-x-m15"+suffix,
		"العربية M15 "+suffix,
		"rtl",
	)
	return fixtures
}

func m15AssertTranslationIdentityAndRoundTrip(
	t *testing.T,
	ctx context.Context,
	db *sql.DB,
	cfg config.DatabaseConfig,
	fixtures m15Fixtures,
) {
	t.Helper()

	records := []struct {
		localeID any
		category string
		key      string
		value    string
		source   string
	}{
		{fixtures.english, "common", "welcome", "Hello, {name}!", "editor"},
		{fixtures.thai, "common", "welcome", "สวัสดี {name}!", "import"},
		{
			fixtures.thai,
			"common",
			"item-count",
			"{count, plural, =0 {ไม่มีรายการ} one {# รายการ} other {# รายการ}}",
			"editor",
		},
		{fixtures.arabic, "navigation", "sign-in", "تسجيل الدخول", "editor"},
		{fixtures.thai, "common", "celebration", "👩🏽‍💻 e\u0301 🎉", "editor"},
		{fixtures.english, "navigation", "welcome", "Hello, {name}!", "editor"},
		{fixtures.english, "catalog2", "welcome2", "ASCII digit grammar", "editor"},
		{fixtures.english, "catalog.section", "account.profile", "Dotted grammar", "editor"},
		{fixtures.english, "catalog_section", "account_profile", "Underscore grammar", "editor"},
		{fixtures.english, "catalog-section", "account-profile", "Hyphen grammar", "editor"},
		{
			fixtures.english,
			"validation.forms",
			"json-like",
			"Line 1\nLine 2: {\"field\": \"email\", \"valid\": false}",
			"import",
		},
	}
	for _, record := range records {
		m15InsertTranslation(
			t,
			ctx,
			db,
			cfg,
			record.localeID,
			record.category,
			record.key,
			record.value,
			record.source,
		)
	}
	require.Equal(t, len(records), m15TranslationCount(t, ctx, db))
	require.Equal(t, len(records), m15ActiveTranslationCount(t, ctx, db))

	for _, expected := range records {
		actual := m15ReadTranslation(
			t,
			ctx,
			db,
			cfg,
			expected.localeID,
			expected.category,
			expected.key,
		)
		require.True(t, actual.value.Valid)
		require.Equal(t, []byte(expected.value), []byte(actual.value.String))
		require.Equal(t, expected.source, actual.source)
		require.Equal(t, int64(1), actual.version)
		require.False(t, actual.createdAt.IsZero())
		require.False(t, actual.updatedAt.Before(actual.createdAt))
	}

	_, err := db.ExecContext(
		ctx,
		m15TranslationInsertSQL(cfg),
		fixtures.english,
		"common",
		"welcome",
		"Duplicate logical identity",
		"editor",
	)
	require.Error(t, err, "the same locale/category/key tuple must be unique")
	m15RequireTranslationCount(t, ctx, db, cfg, fixtures.english, "common", "welcome", 1)
}

func m15TranslationInsertSQL(cfg config.DatabaseConfig) string {
	return "INSERT INTO translations (locale_id, category, translation_key, value, source) VALUES (" +
		migrationPlaceholders(cfg, 5) + ")"
}

func m15InsertTranslation(
	t *testing.T,
	ctx context.Context,
	db *sql.DB,
	cfg config.DatabaseConfig,
	localeID any,
	category string,
	key string,
	value any,
	source string,
) {
	t.Helper()

	_, err := db.ExecContext(
		ctx,
		m15TranslationInsertSQL(cfg),
		localeID,
		category,
		key,
		value,
		source,
	)
	require.NoError(t, err)
}

func m15ReadTranslation(
	t *testing.T,
	ctx context.Context,
	db *sql.DB,
	cfg config.DatabaseConfig,
	localeID any,
	category string,
	key string,
) m15TranslationRecord {
	t.Helper()

	var record m15TranslationRecord
	require.NoError(t, db.QueryRowContext(
		ctx,
		"SELECT value, source, version, created_at, updated_at FROM translations "+
			"WHERE locale_id = "+migrationPlaceholder(cfg, 1)+
			" AND category = "+migrationPlaceholder(cfg, 2)+
			" AND translation_key = "+migrationPlaceholder(cfg, 3),
		localeID,
		category,
		key,
	).Scan(
		&record.value,
		&record.source,
		&record.version,
		&record.createdAt,
		&record.updatedAt,
	))
	return record
}

func m15RequireTranslationCount(
	t *testing.T,
	ctx context.Context,
	db *sql.DB,
	cfg config.DatabaseConfig,
	localeID any,
	category string,
	key string,
	expected int,
) {
	t.Helper()

	var count int
	require.NoError(t, db.QueryRowContext(
		ctx,
		"SELECT COUNT(*) FROM translations WHERE locale_id = "+migrationPlaceholder(cfg, 1)+
			" AND category = "+migrationPlaceholder(cfg, 2)+
			" AND translation_key = "+migrationPlaceholder(cfg, 3),
		localeID,
		category,
		key,
	).Scan(&count))
	require.Equal(t, expected, count, fmt.Sprintf("unexpected row count for %s/%s", category, key))
}

func m15ActiveTranslationCount(t *testing.T, ctx context.Context, db *sql.DB) int {
	t.Helper()

	var count int
	require.NoError(t, db.QueryRowContext(
		ctx,
		"SELECT COUNT(*) FROM translations WHERE value IS NOT NULL",
	).Scan(&count))
	return count
}
