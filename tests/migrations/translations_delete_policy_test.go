package migrations_test

import (
	"context"
	"database/sql"
	"testing"

	"github.com/korwlik23/go-api-starter/internal/platform/config"
	"github.com/stretchr/testify/require"
)

func m15AssertTranslationDeletePolicy(
	t *testing.T,
	ctx context.Context,
	db *sql.DB,
	cfg config.DatabaseConfig,
	fixtures m15Fixtures,
	seedOffset byte,
) {
	t.Helper()

	activeLocaleID := m14LocaleID(t, cfg, 140+seedOffset)
	tombstoneLocaleID := m14LocaleID(t, cfg, 141+seedOffset)
	unreferencedLocaleID := m14LocaleID(t, cfg, 142+seedOffset)
	for _, locale := range []struct {
		id  any
		tag string
	}{
		{activeLocaleID, "en-x-fk" + fixtures.suffix},
		{tombstoneLocaleID, "th-x-fk" + fixtures.suffix},
		{unreferencedLocaleID, "de-x-fk" + fixtures.suffix},
	} {
		m14InsertLocale(t, ctx, db, cfg, locale.id, locale.tag, locale.tag, "ltr")
	}

	m15InsertTranslation(
		t,
		ctx,
		db,
		cfg,
		activeLocaleID,
		"foreign-key",
		"active",
		"Active child",
		"editor",
	)
	m15InsertTranslation(
		t,
		ctx,
		db,
		cfg,
		tombstoneLocaleID,
		"foreign-key",
		"tombstone",
		nil,
		"editor",
	)

	unknownLocaleID := m14LocaleID(t, cfg, 150+seedOffset)
	_, err := db.ExecContext(
		ctx,
		m15TranslationInsertSQL(cfg),
		unknownLocaleID,
		"foreign-key",
		"unknown-locale",
		"Unknown parent",
		"editor",
	)
	require.Error(t, err, "an override must reference an existing locale")
	m15RequireTranslationCount(t, ctx, db, cfg, unknownLocaleID, "foreign-key", "unknown-locale", 0)

	_, err = db.ExecContext(
		ctx,
		"UPDATE locales SET id = "+migrationPlaceholder(cfg, 1)+
			" WHERE id = "+migrationPlaceholder(cfg, 2),
		m14LocaleID(t, cfg, 151+seedOffset),
		activeLocaleID,
	)
	require.Error(t, err, "a referenced locale identity must remain stable")
	m14RequireLocaleIDCount(t, ctx, db, cfg, activeLocaleID, 1)

	for _, localeID := range []any{activeLocaleID, tombstoneLocaleID} {
		_, err = db.ExecContext(
			ctx,
			"DELETE FROM locales WHERE id = "+migrationPlaceholder(cfg, 1),
			localeID,
		)
		require.Error(t, err, "active overrides and tombstones must both restrict locale deletion")
		m14RequireLocaleIDCount(t, ctx, db, cfg, localeID, 1)
	}
	m15RequireTranslationCount(t, ctx, db, cfg, activeLocaleID, "foreign-key", "active", 1)
	m15RequireTranslationCount(t, ctx, db, cfg, tombstoneLocaleID, "foreign-key", "tombstone", 1)

	_, err = db.ExecContext(
		ctx,
		"DELETE FROM locales WHERE id = "+migrationPlaceholder(cfg, 1),
		unreferencedLocaleID,
	)
	require.NoError(t, err, "an isolated non-default locale must remain deletable at the DDL layer")
	m14RequireLocaleIDCount(t, ctx, db, cfg, unreferencedLocaleID, 0)

	m15PurgeTranslationForParentDeleteControl(
		t,
		ctx,
		db,
		cfg,
		activeLocaleID,
		"foreign-key",
		"active",
	)
	_, err = db.ExecContext(
		ctx,
		"DELETE FROM locales WHERE id = "+migrationPlaceholder(cfg, 1),
		activeLocaleID,
	)
	require.NoError(t, err, "parent deletion may proceed only after an explicit child purge")
	m14RequireLocaleIDCount(t, ctx, db, cfg, activeLocaleID, 0)
	m15RequireTranslationCount(t, ctx, db, cfg, tombstoneLocaleID, "foreign-key", "tombstone", 1)
}

func m15PurgeTranslationForParentDeleteControl(
	t *testing.T,
	ctx context.Context,
	db *sql.DB,
	cfg config.DatabaseConfig,
	localeID any,
	category string,
	key string,
) {
	t.Helper()

	result, err := db.ExecContext(
		ctx,
		"DELETE FROM translations WHERE locale_id = "+migrationPlaceholder(cfg, 1)+
			" AND category = "+migrationPlaceholder(cfg, 2)+
			" AND translation_key = "+migrationPlaceholder(cfg, 3),
		localeID,
		category,
		key,
	)
	require.NoError(t, err)
	m15RequireAffectedRows(t, result, 1)
}
