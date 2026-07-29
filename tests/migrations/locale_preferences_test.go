package migrations_test

import (
	"context"
	"database/sql"
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
	m16PreferencesTable       = "user_locale_preferences"
	m16PreferenceLocalesTable = "user_locale_preference_locales"
	m16MigrationFilename      = "000003_create_user_locale_preferences.sql"
)

func TestM16(t *testing.T) {
	ctx, cfg, sqlDB := openMigrationTestDatabase(t)
	m16CleanupSchema(t, sqlDB)
	t.Cleanup(func() { m16CleanupSchema(t, sqlDB) })

	identityProvider := newMigrationTestProvider(
		t,
		sqlDB,
		cfg,
		migrationTestFileBundle(
			t,
			"identity",
			migrationTestDialect(t, cfg),
			m3MigrationFilename,
			false,
		),
		m3VersionTable,
	)
	localizationProvider := newMigrationTestProvider(
		t,
		sqlDB,
		cfg,
		migrationTestFilesBundleWithOptionalLast(
			t,
			"localization",
			migrationTestDialect(t, cfg),
			m14MigrationFilename,
			m15MigrationFilename,
			m16MigrationFilename,
		),
		m14MigrationTableName,
	)

	_, err := identityProvider.UpTo(ctx, 1)
	require.NoError(t, err)
	_, err = localizationProvider.UpTo(ctx, 2)
	require.NoError(t, err)
	m16CreatePreexistingParents(t, ctx, sqlDB, cfg)

	_, err = localizationProvider.UpTo(ctx, 3)
	require.NoError(t, err)
	m16AssertPreferenceSchemas(t, ctx, sqlDB, cfg)
	m9AssertVersion(t, ctx, localizationProvider, 3)
	m3AssertVersion(t, ctx, identityProvider, 1)
	m16AssertNoSeededPreferences(t, ctx, sqlDB)
	m16AssertPreferenceBehavior(t, ctx, sqlDB, cfg, 0)
	m16AssertPreferenceConstraints(t, ctx, sqlDB, cfg, 0)
	m16AssertPreferenceState(t, ctx, sqlDB, cfg, 0)
	m16AssertPreferenceDeletePolicy(t, ctx, sqlDB, cfg, 0)

	usersSnapshot := m16SnapshotUsers(t, ctx, sqlDB)
	localesSnapshot := m15SnapshotLocales(t, ctx, sqlDB)
	translationsSnapshot := m16SnapshotTranslations(t, ctx, sqlDB)

	_, err = localizationProvider.DownTo(ctx, 2)
	require.NoError(t, err)
	require.False(t, migrationTableExists(t, ctx, sqlDB, cfg, m16PreferenceLocalesTable))
	require.False(t, migrationTableExists(t, ctx, sqlDB, cfg, m16PreferencesTable))
	m16AssertUsersSchemaOnly(t, ctx, sqlDB, cfg)
	m14AssertLocalesSchema(t, ctx, sqlDB, cfg)
	m15AssertTranslationsSchema(t, ctx, sqlDB, cfg)
	require.Equal(t, usersSnapshot, m16SnapshotUsers(t, ctx, sqlDB))
	require.Equal(t, localesSnapshot, m15SnapshotLocales(t, ctx, sqlDB))
	require.Equal(t, translationsSnapshot, m16SnapshotTranslations(t, ctx, sqlDB))
	m9AssertVersion(t, ctx, localizationProvider, 2)
	m3AssertVersion(t, ctx, identityProvider, 1)

	_, err = localizationProvider.UpTo(ctx, 3)
	require.NoError(t, err)
	m16AssertPreferenceSchemas(t, ctx, sqlDB, cfg)
	m16AssertNoSeededPreferences(t, ctx, sqlDB)
	m16AssertPreferenceBehavior(t, ctx, sqlDB, cfg, 100)
	m16AssertPreferenceConstraints(t, ctx, sqlDB, cfg, 100)
	m16AssertPreferenceState(t, ctx, sqlDB, cfg, 100)
	m16AssertPreferenceDeletePolicy(t, ctx, sqlDB, cfg, 100)
	m9AssertVersion(t, ctx, localizationProvider, 3)
}

func TestM16Parity(t *testing.T) {
	t.Parallel()

	module := database.MigrationModule{
		ID:      "localization",
		Bundles: strictMigrationTestBundles(t, "localization"),
	}
	require.NoError(t, app.ValidateMigrationDialectParity(
		[]database.MigrationModule{module},
	))
}

func TestM16MySQLCheckBoundLocaleUsesImplicitRestrictiveActions(t *testing.T) {
	t.Parallel()

	bundle := migrationTestFileBundle(
		t,
		"localization",
		"mysql",
		m16MigrationFilename,
		true,
	)
	migration, err := fs.ReadFile(bundle, m16MigrationFilename)
	require.NoError(t, err)
	source := strings.ToUpper(string(migration))
	require.Contains(t, source, "CONSTRAINT USER_LOCALE_PREFERENCE_LOCALES_LOCALE_FK FOREIGN KEY (LOCALE_ID)")
	localeForeignKey := source[strings.Index(source, "CONSTRAINT USER_LOCALE_PREFERENCE_LOCALES_LOCALE_FK"):]
	require.NotContains(t, localeForeignKey, "ON UPDATE")
	require.NotContains(t, localeForeignKey, "ON DELETE")
}

func m16AssertPreferenceSchemas(
	t *testing.T,
	ctx context.Context,
	db *sql.DB,
	cfg config.DatabaseConfig,
) {
	t.Helper()
	m16AssertPreferencesSchema(t, ctx, db, cfg)
	m16AssertPreferenceLocalesSchema(t, ctx, db, cfg)
}

func m16AssertUsersSchemaOnly(
	t *testing.T,
	ctx context.Context,
	db *sql.DB,
	cfg config.DatabaseConfig,
) {
	t.Helper()

	columns := migrationColumns(t, ctx, db, cfg, m3UsersTable)
	require.NotEmpty(t, columns, "users table must exist")
	require.Len(t, columns, 6, "M3 users schema must survive M16 rollback unchanged")
	m9AssertColumn(t, columns, "id", m3ExpectedIDType(t, cfg), "NO", false)
	m9AssertColumn(t, columns, "normalized_email", m3ExpectedVarcharType(t, cfg), "NO", false)
	m9AssertColumn(t, columns, "status", m3ExpectedVarcharType(t, cfg), "NO", false)
	m9AssertColumn(t, columns, "authorization_version", "bigint", "NO", true)
	m9AssertColumn(t, columns, "created_at", m3ExpectedTimestampType(t, cfg), "NO", true)
	m9AssertColumn(t, columns, "updated_at", m3ExpectedTimestampType(t, cfg), "NO", true)
	require.Equal(t, int64(320), columns["normalized_email"].maximumLength.Int64)
	if cfg.Driver == "mysql" {
		require.Equal(t, int64(16), columns["id"].maximumLength.Int64)
	}
	require.True(t, migrationHasExactConstraintColumns(
		t, ctx, db, cfg, m3UsersTable, "PRIMARY KEY", []string{"id"},
	))
	require.True(t, migrationHasExactConstraintColumns(
		t, ctx, db, cfg, m3UsersTable, "UNIQUE", []string{"normalized_email"},
	))
}

func m16AssertPreferencesSchema(
	t *testing.T,
	ctx context.Context,
	db *sql.DB,
	cfg config.DatabaseConfig,
) {
	t.Helper()

	columns := migrationColumns(t, ctx, db, cfg, m16PreferencesTable)
	require.NotEmpty(t, columns, "user locale preferences table must exist")
	require.Len(t, columns, 5, "M16 preference header must contain only owner, mode, and version state")
	m9AssertColumn(t, columns, "user_id", m3ExpectedIDType(t, cfg), "NO", false)
	m9AssertColumn(t, columns, "all_languages", m9ExpectedBooleanType(t, cfg), "NO", false)
	m9AssertColumn(t, columns, "version", "bigint", "NO", true)
	m9AssertColumn(t, columns, "created_at", m3ExpectedTimestampType(t, cfg), "NO", true)
	m9AssertColumn(t, columns, "updated_at", m3ExpectedTimestampType(t, cfg), "NO", true)
	if cfg.Driver == "mysql" {
		require.Equal(t, int64(16), columns["user_id"].maximumLength.Int64)
	}

	require.True(t, migrationHasExactConstraintColumns(
		t, ctx, db, cfg, m16PreferencesTable, "PRIMARY KEY", []string{"user_id"},
	))
	indexes := migrationIndexes(t, ctx, db, cfg, m16PreferencesTable)
	require.Len(t, indexes, 1, "preference header needs only its primary-key index")
	require.Len(t, migrationForeignKeys(t, ctx, db, cfg, m16PreferencesTable), 1)
	require.True(t, migrationHasExactForeignKey(
		t,
		ctx,
		db,
		cfg,
		m16PreferencesTable,
		migrationForeignKey{
			localColumns:      []string{"user_id"},
			referencedTable:   m3UsersTable,
			referencedColumns: []string{"id"},
			updateRule:        "RESTRICT",
			deleteRule:        "CASCADE",
		},
	))
}

func m16AssertPreferenceLocalesSchema(
	t *testing.T,
	ctx context.Context,
	db *sql.DB,
	cfg config.DatabaseConfig,
) {
	t.Helper()

	columns := migrationColumns(t, ctx, db, cfg, m16PreferenceLocalesTable)
	require.NotEmpty(t, columns, "user locale preference selection table must exist")
	require.Len(t, columns, 2, "M16 selection mapping must contain only user and locale identity")
	for _, column := range []string{"user_id", "locale_id"} {
		m9AssertColumn(t, columns, column, m3ExpectedIDType(t, cfg), "NO", false)
		if cfg.Driver == "mysql" {
			require.Equal(t, int64(16), columns[column].maximumLength.Int64)
		}
	}

	require.True(t, migrationHasExactConstraintColumns(
		t,
		ctx,
		db,
		cfg,
		m16PreferenceLocalesTable,
		"PRIMARY KEY",
		[]string{"user_id", "locale_id"},
	))
	indexes := migrationIndexes(t, ctx, db, cfg, m16PreferenceLocalesTable)
	require.Len(t, indexes, 2, "selection mapping needs only primary and reverse-locale indexes")
	require.True(t, migrationHasExactIndexColumns(
		t, ctx, db, cfg, m16PreferenceLocalesTable, false, []string{"locale_id", "user_id"},
	))
	require.Len(t, migrationForeignKeys(t, ctx, db, cfg, m16PreferenceLocalesTable), 2)
	require.True(t, migrationHasExactForeignKey(
		t,
		ctx,
		db,
		cfg,
		m16PreferenceLocalesTable,
		migrationForeignKey{
			localColumns:      []string{"user_id"},
			referencedTable:   m16PreferencesTable,
			referencedColumns: []string{"user_id"},
			updateRule:        "RESTRICT",
			deleteRule:        "CASCADE",
		},
	))
	require.True(t, migrationHasExactForeignKey(
		t,
		ctx,
		db,
		cfg,
		m16PreferenceLocalesTable,
		migrationForeignKey{
			localColumns:      []string{"locale_id"},
			referencedTable:   m14LocalesTable,
			referencedColumns: []string{"id"},
			updateRule:        m10ExpectedRestrictiveRule(cfg),
			deleteRule:        m10ExpectedRestrictiveRule(cfg),
		},
	))
}

func m16CreatePreexistingParents(
	t *testing.T,
	ctx context.Context,
	db *sql.DB,
	cfg config.DatabaseConfig,
) {
	t.Helper()

	userID := m3UserID(t, cfg, 1)
	localeID := m14LocaleID(t, cfg, 1)
	m16InsertUser(t, ctx, db, cfg, userID, "m16-preexisting@example.test")
	m14InsertLocale(t, ctx, db, cfg, localeID, "x-m16pre", "M16 Pre-existing", "ltr")
	m15InsertTranslation(t, ctx, db, cfg, localeID, "common", "preexisting", "Pre-existing", "editor")
}

func m16AssertNoSeededPreferences(t *testing.T, ctx context.Context, db *sql.DB) {
	t.Helper()
	require.Zero(t, m16TableCount(t, ctx, db, m16PreferencesTable))
	require.Zero(t, m16TableCount(t, ctx, db, m16PreferenceLocalesTable))
}

type m16UserSnapshot struct {
	id                   string
	normalizedEmail      string
	status               string
	authorizationVersion int64
	createdAt            time.Time
	updatedAt            time.Time
}

func m16SnapshotUsers(t *testing.T, ctx context.Context, db *sql.DB) []m16UserSnapshot {
	t.Helper()

	rows, err := db.QueryContext(
		ctx,
		"SELECT id, normalized_email, status, authorization_version, created_at, updated_at "+
			"FROM users ORDER BY normalized_email",
	)
	require.NoError(t, err)
	defer func() { require.NoError(t, rows.Close()) }()

	var snapshot []m16UserSnapshot
	for rows.Next() {
		var id any
		var record m16UserSnapshot
		require.NoError(t, rows.Scan(
			&id,
			&record.normalizedEmail,
			&record.status,
			&record.authorizationVersion,
			&record.createdAt,
			&record.updatedAt,
		))
		record.id = m15SnapshotID(id)
		snapshot = append(snapshot, record)
	}
	require.NoError(t, rows.Err())
	return snapshot
}

type m16TranslationSnapshot struct {
	localeID       string
	category       string
	translationKey string
	value          sql.NullString
	source         string
	version        int64
	createdAt      time.Time
	updatedAt      time.Time
}

func m16SnapshotTranslations(t *testing.T, ctx context.Context, db *sql.DB) []m16TranslationSnapshot {
	t.Helper()

	rows, err := db.QueryContext(
		ctx,
		"SELECT locale_id, category, translation_key, value, source, version, created_at, updated_at "+
			"FROM translations ORDER BY category, translation_key, locale_id",
	)
	require.NoError(t, err)
	defer func() { require.NoError(t, rows.Close()) }()

	var snapshot []m16TranslationSnapshot
	for rows.Next() {
		var localeID any
		var record m16TranslationSnapshot
		require.NoError(t, rows.Scan(
			&localeID,
			&record.category,
			&record.translationKey,
			&record.value,
			&record.source,
			&record.version,
			&record.createdAt,
			&record.updatedAt,
		))
		record.localeID = m15SnapshotID(localeID)
		snapshot = append(snapshot, record)
	}
	require.NoError(t, rows.Err())
	return snapshot
}

func m16TableCount(t *testing.T, ctx context.Context, db *sql.DB, table string) int {
	t.Helper()
	var count int
	require.NoError(t, db.QueryRowContext(ctx, "SELECT COUNT(*) FROM "+table).Scan(&count))
	return count
}

func m16CleanupSchema(t *testing.T, db *sql.DB) {
	t.Helper()

	cleanupContext, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	for _, table := range []string{
		m16PreferenceLocalesTable,
		m16PreferencesTable,
		m15TranslationsTable,
		m14LocalesTable,
		m14MigrationTableName,
		m3UsersTable,
		m3VersionTable,
	} {
		_, err := db.ExecContext(cleanupContext, "DROP TABLE IF EXISTS "+table)
		require.NoError(t, err)
	}
}
