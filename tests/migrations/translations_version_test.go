package migrations_test

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/korwlik23/go-api-starter/internal/platform/config"
	"github.com/stretchr/testify/require"
)

func m15AssertTranslationVersionLifecycle(
	t *testing.T,
	ctx context.Context,
	db *sql.DB,
	cfg config.DatabaseConfig,
	fixtures m15Fixtures,
) {
	t.Helper()

	const (
		category = "versioning"
		key      = "message"
		valueA   = "State A"
	)
	baseTotal := m15TranslationCount(t, ctx, db)
	baseActive := m15ActiveTranslationCount(t, ctx, db)
	m15InsertTranslation(t, ctx, db, cfg, fixtures.english, category, key, valueA, "editor")
	initial := m15ReadTranslation(t, ctx, db, cfg, fixtures.english, category, key)
	require.True(t, initial.value.Valid)
	require.Equal(t, valueA, initial.value.String)
	require.Equal(t, "editor", initial.source)
	require.Equal(t, int64(1), initial.version)
	require.Equal(t, baseTotal+1, m15TranslationCount(t, ctx, db))
	require.Equal(t, baseActive+1, m15ActiveTranslationCount(t, ctx, db))

	resetAt := initial.updatedAt.Add(time.Second)
	result, err := db.ExecContext(
		ctx,
		"UPDATE translations SET value = NULL, source = "+migrationPlaceholder(cfg, 1)+
			", version = version + 1, updated_at = "+migrationPlaceholder(cfg, 2)+
			" WHERE locale_id = "+migrationPlaceholder(cfg, 3)+
			" AND category = "+migrationPlaceholder(cfg, 4)+
			" AND translation_key = "+migrationPlaceholder(cfg, 5)+
			" AND version = "+migrationPlaceholder(cfg, 6),
		"editor",
		resetAt,
		fixtures.english,
		category,
		key,
		1,
	)
	require.NoError(t, err)
	m15RequireAffectedRows(t, result, 1)
	tombstone := m15ReadTranslation(t, ctx, db, cfg, fixtures.english, category, key)
	require.False(t, tombstone.value.Valid)
	require.Equal(t, "editor", tombstone.source)
	require.Equal(t, int64(2), tombstone.version)
	require.Equal(t, initial.createdAt, tombstone.createdAt)
	require.WithinDuration(t, resetAt, tombstone.updatedAt, time.Microsecond)
	require.Equal(t, baseTotal+1, m15TranslationCount(t, ctx, db))
	require.Equal(t, baseActive, m15ActiveTranslationCount(t, ctx, db))

	staleAt := resetAt.Add(time.Second)
	result, err = db.ExecContext(
		ctx,
		"UPDATE translations SET value = "+migrationPlaceholder(cfg, 1)+
			", source = "+migrationPlaceholder(cfg, 2)+
			", version = version + 1, updated_at = "+migrationPlaceholder(cfg, 3)+
			" WHERE locale_id = "+migrationPlaceholder(cfg, 4)+
			" AND category = "+migrationPlaceholder(cfg, 5)+
			" AND translation_key = "+migrationPlaceholder(cfg, 6)+
			" AND version = "+migrationPlaceholder(cfg, 7),
		"Stale write",
		"import",
		staleAt,
		fixtures.english,
		category,
		key,
		1,
	)
	require.NoError(t, err)
	m15RequireAffectedRows(t, result, 0)
	require.Equal(t, tombstone, m15ReadTranslation(t, ctx, db, cfg, fixtures.english, category, key))

	_, err = db.ExecContext(
		ctx,
		m15TranslationInsertSQL(cfg),
		fixtures.english,
		category,
		key,
		valueA,
		"import",
	)
	require.Error(t, err, "a tombstone must retain the logical identity and prevent version reset")
	m15RequireTranslationCount(t, ctx, db, cfg, fixtures.english, category, key, 1)

	recreateAt := resetAt.Add(2 * time.Second)
	result, err = db.ExecContext(
		ctx,
		"UPDATE translations SET value = "+migrationPlaceholder(cfg, 1)+
			", source = "+migrationPlaceholder(cfg, 2)+
			", version = version + 1, updated_at = "+migrationPlaceholder(cfg, 3)+
			" WHERE locale_id = "+migrationPlaceholder(cfg, 4)+
			" AND category = "+migrationPlaceholder(cfg, 5)+
			" AND translation_key = "+migrationPlaceholder(cfg, 6)+
			" AND version = "+migrationPlaceholder(cfg, 7),
		valueA,
		"import",
		recreateAt,
		fixtures.english,
		category,
		key,
		2,
	)
	require.NoError(t, err)
	m15RequireAffectedRows(t, result, 1)
	recreated := m15ReadTranslation(t, ctx, db, cfg, fixtures.english, category, key)
	require.True(t, recreated.value.Valid)
	require.Equal(t, valueA, recreated.value.String)
	require.Equal(t, "import", recreated.source)
	require.Equal(t, int64(3), recreated.version)
	require.Equal(t, initial.createdAt, recreated.createdAt)
	require.WithinDuration(t, recreateAt, recreated.updatedAt, time.Microsecond)
	require.Equal(t, baseActive+1, m15ActiveTranslationCount(t, ctx, db))

	result, err = db.ExecContext(
		ctx,
		"UPDATE translations SET value = NULL, version = version + 1 "+
			"WHERE locale_id = "+migrationPlaceholder(cfg, 1)+
			" AND category = "+migrationPlaceholder(cfg, 2)+
			" AND translation_key = "+migrationPlaceholder(cfg, 3)+
			" AND version = "+migrationPlaceholder(cfg, 4),
		fixtures.english,
		category,
		key,
		2,
	)
	require.NoError(t, err)
	m15RequireAffectedRows(t, result, 0)
	require.Equal(t, recreated, m15ReadTranslation(t, ctx, db, cfg, fixtures.english, category, key))
}

func m15RequireAffectedRows(t *testing.T, result sql.Result, expected int64) {
	t.Helper()

	affected, err := result.RowsAffected()
	require.NoError(t, err)
	require.Equal(t, expected, affected)
}
