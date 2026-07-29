package migrations_test

import (
	"context"
	"database/sql"
	"strings"
	"testing"
	"time"

	"github.com/korwlik23/go-api-starter/internal/platform/config"
	"github.com/stretchr/testify/require"
)

func m15AssertTranslationConstraints(
	t *testing.T,
	ctx context.Context,
	db *sql.DB,
	cfg config.DatabaseConfig,
	fixtures m15Fixtures,
) {
	t.Helper()

	m15AssertRequiredTranslationIdentity(t, ctx, db, cfg, fixtures.english)
	m15AssertCategoryConstraints(t, ctx, db, cfg, fixtures.english)
	m15AssertTranslationKeyConstraints(t, ctx, db, cfg, fixtures.english)
	m15AssertTranslationValueConstraints(t, ctx, db, cfg, fixtures.english)
	m15AssertTranslationSourceAndState(t, ctx, db, cfg, fixtures.english)
	m15AssertMySQLNonStrictTranslationBoundaries(t, ctx, db, cfg, fixtures.english)
}

func m15AssertRequiredTranslationIdentity(
	t *testing.T,
	ctx context.Context,
	db *sql.DB,
	cfg config.DatabaseConfig,
	localeID any,
) {
	t.Helper()

	baseCount := m15TranslationCount(t, ctx, db)
	tests := []struct {
		name      string
		statement string
		arguments []any
	}{
		{
			name:      "locale_id",
			statement: "INSERT INTO translations (category, translation_key, value, source) VALUES (" + migrationPlaceholders(cfg, 4) + ")",
			arguments: []any{"required", "locale", "Missing locale", "editor"},
		},
		{
			name:      "category",
			statement: "INSERT INTO translations (locale_id, translation_key, value, source) VALUES (" + migrationPlaceholders(cfg, 4) + ")",
			arguments: []any{localeID, "missing-category", "Missing category", "editor"},
		},
		{
			name:      "translation_key",
			statement: "INSERT INTO translations (locale_id, category, value, source) VALUES (" + migrationPlaceholders(cfg, 4) + ")",
			arguments: []any{localeID, "required", "Missing key", "editor"},
		},
		{
			name:      "source",
			statement: "INSERT INTO translations (locale_id, category, translation_key, value) VALUES (" + migrationPlaceholders(cfg, 4) + ")",
			arguments: []any{localeID, "required", "missing-source", "Missing source"},
		},
	}
	for _, test := range tests {
		_, err := db.ExecContext(ctx, test.statement, test.arguments...)
		require.Error(t, err, "%s must be explicit", test.name)
		require.Equal(t, baseCount, m15TranslationCount(t, ctx, db))
	}
}

func m15AssertCategoryConstraints(
	t *testing.T,
	ctx context.Context,
	db *sql.DB,
	cfg config.DatabaseConfig,
	localeID any,
) {
	t.Helper()

	baseCount := m15TranslationCount(t, ctx, db)
	maximumCategory := "c" + strings.Repeat("a", 63)
	require.Len(t, maximumCategory, 64)
	m15InsertTranslation(t, ctx, db, cfg, localeID, maximumCategory, "boundary", "Category boundary", "editor")
	m15RequireTranslationCount(t, ctx, db, cfg, localeID, maximumCategory, "boundary", 1)
	m15DeleteTranslation(t, ctx, db, cfg, localeID, maximumCategory, "boundary")

	invalidCategories := []string{
		"",
		"Common",
		" common",
		"common ",
		"1common",
		".common",
		"common.",
		"common..auth",
		"common__auth",
		"common--auth",
		"common_-auth",
		"common/auth",
		"common\\auth",
		"common%auth",
		"common\nauth",
		"common\x00auth",
		"сommon",
		"__proto__",
		"prototype",
		"constructor",
		"common.__proto__",
		"common.prototype",
		strings.Repeat("c", 65),
	}
	for index, category := range invalidCategories {
		key := "category-constraint-key"
		_, err := db.ExecContext(
			ctx,
			m15TranslationInsertSQL(cfg),
			localeID,
			category,
			key,
			"Invalid category",
			"editor",
		)
		require.Error(t, err, "invalid category %d must be rejected", index)
		require.Equal(t, baseCount, m15TranslationCount(t, ctx, db))
	}
}

func m15AssertTranslationKeyConstraints(
	t *testing.T,
	ctx context.Context,
	db *sql.DB,
	cfg config.DatabaseConfig,
	localeID any,
) {
	t.Helper()

	baseCount := m15TranslationCount(t, ctx, db)
	maximumKey := "k" + strings.Repeat("a", 254)
	require.Len(t, maximumKey, 255)
	m15InsertTranslation(t, ctx, db, cfg, localeID, "key-boundary", maximumKey, "Key boundary", "editor")
	m15RequireTranslationCount(t, ctx, db, cfg, localeID, "key-boundary", maximumKey, 1)
	m15DeleteTranslation(t, ctx, db, cfg, localeID, "key-boundary", maximumKey)

	invalidKeys := []string{
		"",
		"Welcome",
		" welcome",
		"welcome ",
		"1welcome",
		".welcome",
		"welcome.",
		"common..welcome",
		"common__welcome",
		"common--welcome",
		"common_-welcome",
		"common/__proto__",
		"common\\welcome",
		"common%welcome",
		"common\nwelcome",
		"common\x00welcome",
		"сommon.welcome",
		"__proto__",
		"prototype",
		"constructor",
		"common.__proto__",
		"common.prototype",
		"common.constructor",
		strings.Repeat("k", 256),
	}
	for index, key := range invalidKeys {
		category := "key-constraint"
		_, err := db.ExecContext(
			ctx,
			m15TranslationInsertSQL(cfg),
			localeID,
			category,
			key,
			"Invalid key",
			"editor",
		)
		require.Error(t, err, "invalid translation key %d must be rejected", index)
		require.Equal(t, baseCount, m15TranslationCount(t, ctx, db))
	}
}

func m15AssertTranslationValueConstraints(
	t *testing.T,
	ctx context.Context,
	db *sql.DB,
	cfg config.DatabaseConfig,
	localeID any,
) {
	t.Helper()

	baseTotal := m15TranslationCount(t, ctx, db)
	baseActive := m15ActiveTranslationCount(t, ctx, db)
	maximumValue := strings.Repeat("ก", 5461) + "a"
	require.Len(t, []byte(maximumValue), 16384)
	m15InsertTranslation(t, ctx, db, cfg, localeID, "value-boundary", "maximum", maximumValue, "editor")
	record := m15ReadTranslation(t, ctx, db, cfg, localeID, "value-boundary", "maximum")
	require.True(t, record.value.Valid)
	require.Equal(t, []byte(maximumValue), []byte(record.value.String))
	m15DeleteTranslation(t, ctx, db, cfg, localeID, "value-boundary", "maximum")

	m15InsertTranslation(t, ctx, db, cfg, localeID, "value-boundary", "tombstone", nil, "editor")
	tombstone := m15ReadTranslation(t, ctx, db, cfg, localeID, "value-boundary", "tombstone")
	require.False(t, tombstone.value.Valid)
	require.Equal(t, baseTotal+1, m15TranslationCount(t, ctx, db))
	require.Equal(t, baseActive, m15ActiveTranslationCount(t, ctx, db))
	m15DeleteTranslation(t, ctx, db, cfg, localeID, "value-boundary", "tombstone")

	invalidValues := []struct {
		key   string
		value string
	}{
		{key: "invalid-value-empty", value: ""},
		{key: "invalid-value-large", value: maximumValue + "a"},
	}
	for index, test := range invalidValues {
		_, err := db.ExecContext(
			ctx,
			m15TranslationInsertSQL(cfg),
			localeID,
			"value-boundary",
			test.key,
			test.value,
			"editor",
		)
		require.Error(t, err, "invalid active value %d must be rejected", index)
		m15RequireTranslationCount(t, ctx, db, cfg, localeID, "value-boundary", test.key, 0)
	}
	require.Equal(t, baseTotal, m15TranslationCount(t, ctx, db))
}

func m15AssertTranslationSourceAndState(
	t *testing.T,
	ctx context.Context,
	db *sql.DB,
	cfg config.DatabaseConfig,
	localeID any,
) {
	t.Helper()

	baseCount := m15TranslationCount(t, ctx, db)
	invalidSources := []string{"", "bundle", "fallback", "manual", "EDITOR", "editor ", "import ", "editor\x00"}
	for index, source := range invalidSources {
		key := "invalid-source-key"
		_, err := db.ExecContext(
			ctx,
			m15TranslationInsertSQL(cfg),
			localeID,
			"source-constraint",
			key,
			"Invalid source",
			source,
		)
		require.Error(t, err, "invalid source %d must be rejected", index)
		m15RequireTranslationCount(t, ctx, db, cfg, localeID, "source-constraint", key, 0)
	}

	createdAt := time.Now().UTC().Truncate(time.Microsecond)
	for _, version := range []int64{0, -1} {
		key := "invalid-version-key"
		_, err := db.ExecContext(
			ctx,
			"INSERT INTO translations (locale_id, category, translation_key, value, source, version) VALUES ("+
				migrationPlaceholders(cfg, 6)+")",
			localeID,
			"state-constraint",
			key,
			"Invalid version",
			"editor",
			version,
		)
		require.Error(t, err)
		m15RequireTranslationCount(t, ctx, db, cfg, localeID, "state-constraint", key, 0)
	}
	_, err := db.ExecContext(
		ctx,
		"INSERT INTO translations (locale_id, category, translation_key, value, source, created_at, updated_at) VALUES ("+
			migrationPlaceholders(cfg, 7)+")",
		localeID,
		"state-constraint",
		"invalid-time",
		"Invalid time",
		"editor",
		createdAt,
		createdAt.Add(-time.Second),
	)
	require.Error(t, err)
	m15RequireTranslationCount(t, ctx, db, cfg, localeID, "state-constraint", "invalid-time", 0)
	require.Equal(t, baseCount, m15TranslationCount(t, ctx, db))
}

func m15AssertMySQLNonStrictTranslationBoundaries(
	t *testing.T,
	ctx context.Context,
	db *sql.DB,
	cfg config.DatabaseConfig,
	localeID any,
) {
	t.Helper()
	if cfg.Driver != "mysql" {
		return
	}

	connection, err := db.Conn(ctx)
	require.NoError(t, err)
	var previousSQLMode string
	require.NoError(t, connection.QueryRowContext(ctx, "SELECT @@SESSION.sql_mode").Scan(&previousSQLMode))
	require.NoError(t, m9SetSQLMode(ctx, connection, ""))
	defer func() {
		restoreContext, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		require.NoError(t, m9SetSQLMode(restoreContext, connection, previousSQLMode))
		require.NoError(t, connection.Close())
	}()

	baseCount := m15TranslationCountOnQueryer(t, ctx, connection)
	tests := []struct {
		category string
		key      string
		value    string
		source   string
	}{
		{strings.Repeat("c", 65), "semantic-category", "Invalid category", "editor"},
		{strings.Repeat("c", 129), "physical-category", "Invalid category", "editor"},
		{"nonstrict", strings.Repeat("k", 256), "Invalid key", "editor"},
		{"nonstrict", strings.Repeat("k", 513), "Invalid key", "editor"},
		{"nonstrict", "semantic-value", strings.Repeat("v", 16385), "editor"},
		{"nonstrict", "physical-value", strings.Repeat("v", 65536), "editor"},
		{"nonstrict", "physical-source", "Invalid source", "editor" + strings.Repeat("x", 27)},
	}
	for index, test := range tests {
		_, err = connection.ExecContext(
			ctx,
			m15TranslationInsertSQL(cfg),
			localeID,
			test.category,
			test.key,
			test.value,
			test.source,
		)
		require.Error(t, err, "non-strict boundary %d must be rejected", index)
		require.Equal(t, baseCount, m15TranslationCountOnQueryer(t, ctx, connection))
	}
}

type m15Queryer interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

func m15TranslationCountOnQueryer(t *testing.T, ctx context.Context, queryer m15Queryer) int {
	t.Helper()

	var count int
	require.NoError(t, queryer.QueryRowContext(ctx, "SELECT COUNT(*) FROM translations").Scan(&count))
	return count
}

func m15DeleteTranslation(
	t *testing.T,
	ctx context.Context,
	db *sql.DB,
	cfg config.DatabaseConfig,
	localeID any,
	category string,
	key string,
) {
	t.Helper()

	_, err := db.ExecContext(
		ctx,
		"DELETE FROM translations WHERE locale_id = "+migrationPlaceholder(cfg, 1)+
			" AND category = "+migrationPlaceholder(cfg, 2)+
			" AND translation_key = "+migrationPlaceholder(cfg, 3),
		localeID,
		category,
		key,
	)
	require.NoError(t, err)
}
