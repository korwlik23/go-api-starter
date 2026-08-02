package migrations_test

import (
	"context"
	"database/sql"
	"io/fs"
	"testing"
	"time"

	"github.com/korwlik23/go-api-starter/internal/app"
	"github.com/korwlik23/go-api-starter/internal/platform/config"
	"github.com/korwlik23/go-api-starter/internal/platform/database"
	"github.com/stretchr/testify/require"
)

func TestM3A2PublishingMigrationCatalog(t *testing.T) {
	t.Parallel()

	module := database.MigrationModule{
		ID:           "publishing",
		Dependencies: []string{"accounts", "localization"},
		Bundles:      strictMigrationTestBundles(t, "publishing"),
	}
	require.NoError(t, app.ValidateMigrationDialectParity([]database.MigrationModule{module}))

	expected := map[string][]string{
		"000001_create_content_items.sql": {
			"CREATE TABLE content_items",
			"content_items_account_updated_id_index",
			"content_items_account_fk",
		},
		"000002_create_content_translations.sql": {
			"CREATE TABLE content_translations",
			"content_translations_locale_path_status_index",
			"content_translations_content_fk",
			"content_translations_locale_fk",
		},
		"000003_create_taxonomies.sql": {
			"CREATE TABLE taxonomies",
			"taxonomies_account_kind_updated_id_index",
			"taxonomies_account_fk",
		},
		"000004_create_content_taxonomy_terms.sql": {
			"CREATE TABLE content_taxonomy_terms",
			"content_taxonomy_terms_content_fk",
			"content_taxonomy_terms_taxonomy_fk",
		},
		"000005_create_content_revisions.sql": {
			"CREATE TABLE content_revisions",
			"content_revisions_content_created_id_index",
			"content_revisions_content_fk",
			"content_revisions_translation_fk",
		},
	}

	for dialect, bundle := range module.Bundles {
		for filename, markers := range expected {
			content, err := fs.ReadFile(bundle, filename)
			require.NoError(t, err, "%s/%s", dialect, filename)
			require.Contains(t, string(content), "-- +goose Up", "%s/%s", dialect, filename)
			require.Contains(t, string(content), "-- +goose Down", "%s/%s", dialect, filename)
			for _, marker := range markers {
				require.Contains(t, string(content), marker, "%s/%s", dialect, filename)
			}
		}
	}
}

func TestM3A2PublishingSchemaUpDown(t *testing.T) {
	ctx, cfg, sqlDB := openMigrationTestDatabase(t)
	m3CleanupPublishingSchema(t, sqlDB)
	m3CreatePublishingParents(t, ctx, sqlDB, cfg)
	t.Cleanup(func() { m3CleanupPublishingSchema(t, sqlDB) })

	provider := newMigrationTestProvider(
		t,
		sqlDB,
		cfg,
		strictMigrationTestBundles(t, "publishing")[database.MigrationDialect(
			migrationTestDialect(t, cfg),
		)],
		"goose_publishing_version",
	)
	_, err := provider.Up(ctx)
	require.NoError(t, err)

	for _, table := range []string{
		"content_items",
		"content_translations",
		"taxonomies",
		"content_taxonomy_terms",
		"content_revisions",
	} {
		require.True(t, migrationTableExists(t, ctx, sqlDB, cfg, table), table)
	}
	require.True(t, migrationHasExactIndexColumns(
		t, ctx, sqlDB, cfg, "content_items", false,
		[]string{"account_id", "updated_at", "id"},
	))
	require.True(t, migrationHasExactIndexColumns(
		t, ctx, sqlDB, cfg, "content_translations", false,
		[]string{"locale_id", "path", "status"},
	))
	require.True(t, migrationHasExactIndexColumns(
		t, ctx, sqlDB, cfg, "taxonomies", false,
		[]string{"account_id", "kind", "updated_at", "id"},
	))

	_, err = provider.DownTo(ctx, 0)
	require.NoError(t, err)
	for _, table := range []string{
		"content_revisions",
		"content_taxonomy_terms",
		"taxonomies",
		"content_translations",
		"content_items",
	} {
		require.False(t, migrationTableExists(t, ctx, sqlDB, cfg, table), table)
	}
}

func m3CleanupPublishingSchema(t *testing.T, db *sql.DB) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	for _, table := range []string{
		"content_revisions",
		"content_taxonomy_terms",
		"taxonomies",
		"content_translations",
		"content_items",
		"goose_publishing_version",
		"locales",
		"accounts",
	} {
		_, err := db.ExecContext(ctx, "DROP TABLE IF EXISTS "+table)
		require.NoError(t, err)
	}
}

func m3CreatePublishingParents(
	t *testing.T,
	ctx context.Context,
	db *sql.DB,
	cfg config.DatabaseConfig,
) {
	t.Helper()
	idType := "UUID"
	if cfg.Driver == "mysql" {
		idType = "BINARY(16)"
	}
	for _, table := range []string{"accounts", "locales"} {
		_, err := db.ExecContext(ctx, "CREATE TABLE "+table+" (id "+idType+" PRIMARY KEY)")
		require.NoError(t, err)
	}
}
