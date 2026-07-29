package migrations_test

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/korwlik23/go-api-starter/internal/platform/config"
	"github.com/stretchr/testify/require"
)

type m14Fixtures struct {
	english  any
	thai     any
	chinese  any
	serbian  any
	private  any
	extended any
}

func m14AssertLocaleBehavior(
	t *testing.T,
	ctx context.Context,
	db *sql.DB,
	cfg config.DatabaseConfig,
	seedOffset byte,
) {
	t.Helper()

	fixtures := m14Fixtures{
		english:  m14LocaleID(t, cfg, 1+seedOffset),
		thai:     m14LocaleID(t, cfg, 2+seedOffset),
		chinese:  m14LocaleID(t, cfg, 3+seedOffset),
		serbian:  m14LocaleID(t, cfg, 4+seedOffset),
		private:  m14LocaleID(t, cfg, 5+seedOffset),
		extended: m14LocaleID(t, cfg, 6+seedOffset),
	}
	for _, locale := range []struct {
		id        any
		tag       string
		name      string
		direction string
	}{
		{id: fixtures.english, tag: "en", name: "English", direction: "ltr"},
		{id: fixtures.thai, tag: "th", name: "ไทย", direction: "ltr"},
		{id: fixtures.chinese, tag: "zh-hant", name: "繁體中文", direction: "ltr"},
		{id: fixtures.serbian, tag: "sr-latn-rs", name: "Srpski", direction: "ltr"},
		{id: fixtures.private, tag: "x-private", name: "English", direction: "rtl"},
		{
			id:        fixtures.extended,
			tag:       "en-variant1-variant2-a-extend1-x-private1",
			name:      "Extended tag control",
			direction: "ltr",
		},
	} {
		m14InsertLocale(t, ctx, db, cfg, locale.id, locale.tag, locale.name, locale.direction)
	}
	require.Equal(t, 6, m14LocaleCount(t, ctx, db), "the registry must support more than two locales")

	m14AssertLocaleIdentity(t, ctx, db, cfg, fixtures, seedOffset)
	m14AssertRequiredLocaleValues(t, ctx, db, cfg, seedOffset)
	m14AssertSafeDefaults(t, ctx, db, cfg, fixtures.english)
	m14AssertTagConstraints(t, ctx, db, cfg, seedOffset)
	m14AssertNameConstraints(t, ctx, db, cfg, seedOffset)
	m14AssertDirectionConstraints(t, ctx, db, cfg, seedOffset)
	m14AssertMySQLNonStrictLocaleBoundaries(t, ctx, db, cfg, seedOffset)
	m14AssertFallbackConstraints(t, ctx, db, cfg, fixtures, seedOffset)
	m14AssertLocaleStateConstraints(t, ctx, db, cfg, fixtures)
	m14AssertLocaleLifecycle(t, ctx, db, cfg, fixtures.serbian)
}

func m14AssertLocaleIdentity(
	t *testing.T,
	ctx context.Context,
	db *sql.DB,
	cfg config.DatabaseConfig,
	fixtures m14Fixtures,
	seedOffset byte,
) {
	t.Helper()

	_, err := db.ExecContext(
		ctx,
		m14LocaleInsertSQL(cfg),
		m14LocaleID(t, cfg, 7+seedOffset),
		"en",
		"Duplicate tag",
		"ltr",
	)
	require.Error(t, err, "normalized locale tags must be unique")
	_, err = db.ExecContext(
		ctx,
		m14LocaleInsertSQL(cfg),
		fixtures.english,
		"de",
		"Duplicate ID",
		"ltr",
	)
	require.Error(t, err, "locale IDs must be globally unique")
	require.Equal(t, 6, m14LocaleCount(t, ctx, db))
}

func m14AssertRequiredLocaleValues(
	t *testing.T,
	ctx context.Context,
	db *sql.DB,
	cfg config.DatabaseConfig,
	seedOffset byte,
) {
	t.Helper()

	tests := []struct {
		name      string
		statement string
		arguments []any
	}{
		{
			name: "id",
			statement: "INSERT INTO locales (tag, name, direction) VALUES (" +
				migrationPlaceholders(cfg, 3) + ")",
			arguments: []any{"fr", "French", "ltr"},
		},
		{
			name: "tag",
			statement: "INSERT INTO locales (id, name, direction) VALUES (" +
				migrationPlaceholders(cfg, 3) + ")",
			arguments: []any{m14LocaleID(t, cfg, 8+seedOffset), "French", "ltr"},
		},
		{
			name: "name",
			statement: "INSERT INTO locales (id, tag, direction) VALUES (" +
				migrationPlaceholders(cfg, 3) + ")",
			arguments: []any{m14LocaleID(t, cfg, 9+seedOffset), "fr", "ltr"},
		},
		{
			name: "direction",
			statement: "INSERT INTO locales (id, tag, name) VALUES (" +
				migrationPlaceholders(cfg, 3) + ")",
			arguments: []any{m14LocaleID(t, cfg, 10+seedOffset), "fr", "French"},
		},
	}
	for _, test := range tests {
		_, err := db.ExecContext(ctx, test.statement, test.arguments...)
		require.Error(t, err, "%s must be explicit", test.name)
	}
	require.Equal(t, 6, m14LocaleCount(t, ctx, db))
}

func m14AssertSafeDefaults(
	t *testing.T,
	ctx context.Context,
	db *sql.DB,
	cfg config.DatabaseConfig,
	localeID any,
) {
	t.Helper()

	var enabled bool
	var selectable bool
	var fallbackID any
	var defaultSlot sql.NullInt64
	var version int64
	var createdAt time.Time
	var updatedAt time.Time
	require.NoError(t, db.QueryRowContext(
		ctx,
		"SELECT enabled, user_selectable, fallback_locale_id, default_slot, version, created_at, updated_at "+
			"FROM locales WHERE id = "+migrationPlaceholder(cfg, 1),
		localeID,
	).Scan(&enabled, &selectable, &fallbackID, &defaultSlot, &version, &createdAt, &updatedAt))
	require.False(t, enabled, "new locales must fail closed until explicitly enabled")
	require.False(t, selectable)
	require.Nil(t, fallbackID)
	require.False(t, defaultSlot.Valid)
	require.Equal(t, int64(1), version)
	require.False(t, createdAt.IsZero())
	require.False(t, updatedAt.IsZero())
	require.False(t, updatedAt.Before(createdAt))
}

func m14AssertLocaleLifecycle(
	t *testing.T,
	ctx context.Context,
	db *sql.DB,
	cfg config.DatabaseConfig,
	localeID any,
) {
	t.Helper()

	var previousUpdatedAt time.Time
	require.NoError(t, db.QueryRowContext(
		ctx,
		"SELECT updated_at FROM locales WHERE id = "+migrationPlaceholder(cfg, 1),
		localeID,
	).Scan(&previousUpdatedAt))
	updatedAt := previousUpdatedAt.Add(time.Second)
	result, err := db.ExecContext(
		ctx,
		"UPDATE locales SET name = "+migrationPlaceholder(cfg, 1)+
			", direction = "+migrationPlaceholder(cfg, 2)+
			", version = version + 1, updated_at = "+migrationPlaceholder(cfg, 3)+
			" WHERE id = "+migrationPlaceholder(cfg, 4)+
			" AND version = "+migrationPlaceholder(cfg, 5),
		"Српски",
		"rtl",
		updatedAt,
		localeID,
		1,
	)
	require.NoError(t, err)
	affected, err := result.RowsAffected()
	require.NoError(t, err)
	require.Equal(t, int64(1), affected)

	result, err = db.ExecContext(
		ctx,
		"UPDATE locales SET name = "+migrationPlaceholder(cfg, 1)+
			" WHERE id = "+migrationPlaceholder(cfg, 2)+
			" AND version = "+migrationPlaceholder(cfg, 3),
		"Stale write",
		localeID,
		1,
	)
	require.NoError(t, err)
	affected, err = result.RowsAffected()
	require.NoError(t, err)
	require.Zero(t, affected)

	var name string
	var direction string
	var version int64
	var storedUpdatedAt time.Time
	require.NoError(t, db.QueryRowContext(
		ctx,
		"SELECT name, direction, version, updated_at FROM locales WHERE id = "+
			migrationPlaceholder(cfg, 1),
		localeID,
	).Scan(&name, &direction, &version, &storedUpdatedAt))
	require.Equal(t, "Српски", name)
	require.Equal(t, "rtl", direction)
	require.Equal(t, int64(2), version)
	require.WithinDuration(t, updatedAt, storedUpdatedAt, time.Microsecond)
}

func m14LocaleInsertSQL(cfg config.DatabaseConfig) string {
	return "INSERT INTO locales (id, tag, name, direction) VALUES (" +
		migrationPlaceholders(cfg, 4) + ")"
}

func m14InsertLocale(
	t *testing.T,
	ctx context.Context,
	db *sql.DB,
	cfg config.DatabaseConfig,
	id any,
	tag string,
	name string,
	direction string,
) {
	t.Helper()

	_, err := db.ExecContext(ctx, m14LocaleInsertSQL(cfg), id, tag, name, direction)
	require.NoError(t, err)
}
