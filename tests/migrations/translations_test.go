package migrations_test

import (
	"context"
	"database/sql"
	"encoding/hex"
	"fmt"
	"io/fs"
	"strings"
	"testing"
	"time"

	"github.com/korwlik23/go-api-starter/internal/app"
	"github.com/korwlik23/go-api-starter/internal/platform/config"
	"github.com/korwlik23/go-api-starter/internal/platform/database"
	"github.com/stretchr/testify/require"
)

const (
	m15TranslationsTable = "translations"
	m15MigrationFilename = "000002_create_translations.sql"
)

func TestM15(t *testing.T) {
	ctx, cfg, sqlDB := openMigrationTestDatabase(t)
	m15CleanupSchema(t, sqlDB)
	t.Cleanup(func() { m15CleanupSchema(t, sqlDB) })

	provider := newMigrationTestProvider(
		t,
		sqlDB,
		cfg,
		migrationTestFilesBundleWithOptionalLast(
			t,
			"localization",
			migrationTestDialect(t, cfg),
			m14MigrationFilename,
			m15MigrationFilename,
		),
		m14MigrationTableName,
	)

	_, err := provider.UpTo(ctx, 2)
	require.NoError(t, err)
	m15AssertTranslationsSchema(t, ctx, sqlDB, cfg)
	m9AssertVersion(t, ctx, provider, 2)
	m15AssertNoSeededTranslations(t, ctx, sqlDB)
	m15AssertTranslationBehavior(t, ctx, sqlDB, cfg, 0)
	parentSnapshot := m15SnapshotLocales(t, ctx, sqlDB)

	_, err = provider.DownTo(ctx, 1)
	require.NoError(t, err)
	require.False(t, migrationTableExists(t, ctx, sqlDB, cfg, m15TranslationsTable))
	require.True(t, migrationTableExists(t, ctx, sqlDB, cfg, m14LocalesTable))
	m14AssertLocalesSchema(t, ctx, sqlDB, cfg)
	require.Equal(t, parentSnapshot, m15SnapshotLocales(t, ctx, sqlDB))
	m9AssertVersion(t, ctx, provider, 1)

	_, err = provider.UpTo(ctx, 2)
	require.NoError(t, err)
	m15AssertTranslationsSchema(t, ctx, sqlDB, cfg)
	m9AssertVersion(t, ctx, provider, 2)
	m15AssertNoSeededTranslations(t, ctx, sqlDB)
	m15AssertTranslationBehavior(t, ctx, sqlDB, cfg, 100)
}

func TestM15Parity(t *testing.T) {
	t.Parallel()

	module := database.MigrationModule{
		ID:      "localization",
		Bundles: strictMigrationTestBundles(t, "localization"),
	}
	require.NoError(t, app.ValidateMigrationDialectParity(
		[]database.MigrationModule{module},
	))
}

func TestM15MySQLCheckBoundParentUsesImplicitRestrictiveActions(t *testing.T) {
	t.Parallel()

	bundle := migrationTestFileBundle(
		t,
		"localization",
		"mysql",
		m15MigrationFilename,
		true,
	)
	migration, err := fs.ReadFile(bundle, m15MigrationFilename)
	require.NoError(t, err)
	source := strings.ToUpper(string(migration))
	require.Contains(t, source, "CONSTRAINT TRANSLATIONS_LOCALE_FK FOREIGN KEY (LOCALE_ID)")
	require.NotContains(t, source, "ON UPDATE")
	require.NotContains(t, source, "ON DELETE")
}

func m15AssertTranslationsSchema(
	t *testing.T,
	ctx context.Context,
	db *sql.DB,
	cfg config.DatabaseConfig,
) {
	t.Helper()

	columns := migrationColumns(t, ctx, db, cfg, m15TranslationsTable)
	require.NotEmpty(t, columns, "translations table must exist")
	require.Len(t, columns, 8, "M15 must persist only override identity, value, provenance, and version state")
	m9AssertColumn(t, columns, "locale_id", m3ExpectedIDType(t, cfg), "NO", false)
	m9AssertColumn(t, columns, "category", m3ExpectedVarcharType(t, cfg), "NO", false)
	m9AssertColumn(t, columns, "translation_key", m3ExpectedVarcharType(t, cfg), "NO", false)
	m9AssertColumn(t, columns, "value", "text", "YES", false)
	m9AssertColumn(t, columns, "source", m3ExpectedVarcharType(t, cfg), "NO", false)
	m9AssertColumn(t, columns, "version", "bigint", "NO", true)
	m9AssertColumn(t, columns, "created_at", m3ExpectedTimestampType(t, cfg), "NO", true)
	m9AssertColumn(t, columns, "updated_at", m3ExpectedTimestampType(t, cfg), "NO", true)
	if cfg.Driver == "mysql" {
		require.Equal(t, int64(16), columns["locale_id"].maximumLength.Int64)
		require.Equal(t, int64(128), columns["category"].maximumLength.Int64)
		require.Equal(t, int64(512), columns["translation_key"].maximumLength.Int64)
		require.Equal(t, int64(32), columns["source"].maximumLength.Int64)
	} else {
		require.Equal(t, int64(64), columns["category"].maximumLength.Int64)
		require.Equal(t, int64(255), columns["translation_key"].maximumLength.Int64)
		require.Equal(t, int64(16), columns["source"].maximumLength.Int64)
	}

	require.True(t, migrationHasExactConstraintColumns(
		t,
		ctx,
		db,
		cfg,
		m15TranslationsTable,
		"PRIMARY KEY",
		[]string{"locale_id", "category", "translation_key"},
	))
	require.Len(t, migrationForeignKeys(t, ctx, db, cfg, m15TranslationsTable), 1)
	require.True(t, migrationHasExactForeignKey(
		t,
		ctx,
		db,
		cfg,
		m15TranslationsTable,
		migrationForeignKey{
			localColumns:      []string{"locale_id"},
			referencedTable:   m14LocalesTable,
			referencedColumns: []string{"id"},
			updateRule:        m10ExpectedRestrictiveRule(cfg),
			deleteRule:        m10ExpectedRestrictiveRule(cfg),
		},
	))
	indexes := migrationIndexes(t, ctx, db, cfg, m15TranslationsTable)
	require.Len(t, indexes, 1, "the natural primary key must be the only M15 index")
	require.True(t, migrationHasExactIndexColumns(
		t,
		ctx,
		db,
		cfg,
		m15TranslationsTable,
		true,
		[]string{"locale_id", "category", "translation_key"},
	))
}

func m15AssertNoSeededTranslations(t *testing.T, ctx context.Context, db *sql.DB) {
	t.Helper()

	require.Zero(t, m15TranslationCount(t, ctx, db), "M15 must not copy or seed bundled catalog values")
}

func m15TranslationCount(t *testing.T, ctx context.Context, db *sql.DB) int {
	t.Helper()

	var count int
	require.NoError(t, db.QueryRowContext(ctx, "SELECT COUNT(*) FROM translations").Scan(&count))
	return count
}

type m15LocaleSnapshot struct {
	id          string
	tag         string
	name        string
	direction   string
	fallbackID  string
	enabled     bool
	selectable  bool
	defaultSlot sql.NullInt64
	version     int64
	createdAt   time.Time
	updatedAt   time.Time
}

func m15SnapshotLocales(t *testing.T, ctx context.Context, db *sql.DB) []m15LocaleSnapshot {
	t.Helper()

	rows, err := db.QueryContext(
		ctx,
		"SELECT id, tag, name, direction, fallback_locale_id, enabled, user_selectable, "+
			"default_slot, version, created_at, updated_at FROM locales ORDER BY tag",
	)
	require.NoError(t, err)
	defer func() { require.NoError(t, rows.Close()) }()

	snapshot := make([]m15LocaleSnapshot, 0)
	for rows.Next() {
		var row m15LocaleSnapshot
		var rawID any
		var rawFallbackID any
		require.NoError(t, rows.Scan(
			&rawID,
			&row.tag,
			&row.name,
			&row.direction,
			&rawFallbackID,
			&row.enabled,
			&row.selectable,
			&row.defaultSlot,
			&row.version,
			&row.createdAt,
			&row.updatedAt,
		))
		row.id = m15SnapshotID(rawID)
		row.fallbackID = m15SnapshotID(rawFallbackID)
		snapshot = append(snapshot, row)
	}
	require.NoError(t, rows.Err())
	return snapshot
}

func m15SnapshotID(value any) string {
	switch typed := value.(type) {
	case nil:
		return ""
	case []byte:
		return hex.EncodeToString(typed)
	case string:
		return typed
	default:
		return fmt.Sprint(typed)
	}
}

func m15CleanupSchema(t *testing.T, db *sql.DB) {
	t.Helper()

	cleanupContext, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	for _, table := range []string{m15TranslationsTable, m14LocalesTable, m14MigrationTableName} {
		_, err := db.ExecContext(cleanupContext, "DROP TABLE IF EXISTS "+table)
		require.NoError(t, err)
	}
}
