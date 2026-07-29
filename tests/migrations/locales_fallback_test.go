package migrations_test

import (
	"context"
	"database/sql"
	"testing"

	"github.com/korwlik23/go-api-starter/internal/platform/config"
	"github.com/stretchr/testify/require"
)

func m14AssertFallbackConstraints(
	t *testing.T,
	ctx context.Context,
	db *sql.DB,
	cfg config.DatabaseConfig,
	fixtures m14Fixtures,
	seedOffset byte,
) {
	t.Helper()

	for _, sourceID := range []any{fixtures.thai, fixtures.chinese} {
		_, err := db.ExecContext(
			ctx,
			"UPDATE locales SET fallback_locale_id = "+migrationPlaceholder(cfg, 1)+
				" WHERE id = "+migrationPlaceholder(cfg, 2),
			fixtures.english,
			sourceID,
		)
		require.NoError(t, err)
		m14RequireFallback(t, ctx, db, cfg, sourceID, fixtures.english)
	}
	_, err := db.ExecContext(
		ctx,
		"UPDATE locales SET fallback_locale_id = "+migrationPlaceholder(cfg, 1)+
			" WHERE id = "+migrationPlaceholder(cfg, 2),
		fixtures.private,
		fixtures.private,
	)
	require.Error(t, err, "a locale must not directly fall back to itself")
	m14RequireFallback(t, ctx, db, cfg, fixtures.private, nil)
	_, err = db.ExecContext(
		ctx,
		"UPDATE locales SET fallback_locale_id = "+migrationPlaceholder(cfg, 1)+
			" WHERE id = "+migrationPlaceholder(cfg, 2),
		m14LocaleID(t, cfg, 90+seedOffset),
		fixtures.serbian,
	)
	require.Error(t, err, "an unknown fallback locale must be rejected")
	m14RequireFallback(t, ctx, db, cfg, fixtures.serbian, nil)

	_, err = db.ExecContext(
		ctx,
		"DELETE FROM locales WHERE id = "+migrationPlaceholder(cfg, 1),
		fixtures.english,
	)
	require.Error(t, err, "a referenced fallback target must not disappear")
	_, err = db.ExecContext(
		ctx,
		"UPDATE locales SET id = "+migrationPlaceholder(cfg, 1)+
			" WHERE id = "+migrationPlaceholder(cfg, 2),
		m14LocaleID(t, cfg, 91+seedOffset),
		fixtures.english,
	)
	require.Error(t, err, "a referenced fallback identity must remain stable")
	m14RequireLocaleIDCount(t, ctx, db, cfg, fixtures.english, 1)
}

func m14RequireFallback(
	t *testing.T,
	ctx context.Context,
	db *sql.DB,
	cfg config.DatabaseConfig,
	sourceID any,
	expectedFallbackID any,
) {
	t.Helper()

	var fallbackID any
	require.NoError(t, db.QueryRowContext(
		ctx,
		"SELECT fallback_locale_id FROM locales WHERE id = "+migrationPlaceholder(cfg, 1),
		sourceID,
	).Scan(&fallbackID))
	if expectedFallbackID == nil {
		require.Nil(t, fallbackID)
		return
	}

	var count int
	require.NoError(t, db.QueryRowContext(
		ctx,
		"SELECT COUNT(*) FROM locales WHERE id = "+migrationPlaceholder(cfg, 1)+
			" AND fallback_locale_id = "+migrationPlaceholder(cfg, 2),
		sourceID,
		expectedFallbackID,
	).Scan(&count))
	require.Equal(t, 1, count)
}
