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

func TestM3B5DiscoverabilityMigrationCatalog(t *testing.T) {
	t.Parallel()

	module := database.MigrationModule{ID: "discoverability", Dependencies: []string{"accounts", "localization"}, Bundles: strictMigrationTestBundles(t, "discoverability")}
	require.NoError(t, app.ValidateMigrationDialectParity([]database.MigrationModule{module}))
	expected := map[string][]string{
		"000001_create_redirects.sql":      {"CREATE TABLE discoverability_redirects", "discoverability_redirects_account_locale_source_unique", "discoverability_redirects_updated_id_index"},
		"000002_create_seo_defaults.sql":   {"CREATE TABLE discoverability_seo_defaults", "discoverability_seo_defaults_account_locale_unique", "discoverability_seo_defaults_canonical_valid"},
		"000003_create_content_audits.sql": {"CREATE TABLE discoverability_content_audits", "discoverability_content_audits_revision_unique", "discoverability_content_audits_checks_valid"},
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

func TestM3B6DiscoverabilityAuditSchemaUpDown(t *testing.T) {
	ctx, cfg, sqlDB := openMigrationTestDatabase(t)
	m3CleanupDiscoverabilitySchema(t, sqlDB)
	m3CreateDiscoverabilityParents(t, ctx, sqlDB, cfg)
	m3CreateDiscoverabilityAuditParents(t, ctx, sqlDB, cfg)
	t.Cleanup(func() { m3CleanupDiscoverabilitySchema(t, sqlDB) })

	bundles := strictMigrationTestBundles(t, "discoverability")
	dialect := database.MigrationDialect(migrationTestDialect(t, cfg))
	provider := newMigrationTestProvider(t, sqlDB, cfg, bundles[dialect], "goose_discoverability_version")
	_, err := provider.Up(ctx)
	require.NoError(t, err)
	require.True(t, migrationTableExists(t, ctx, sqlDB, cfg, "discoverability_content_audits"))
	require.True(t, migrationHasExactIndexColumns(t, ctx, sqlDB, cfg, "discoverability_content_audits", false, []string{"account_id", "updated_at", "id"}))
	_, err = provider.DownTo(ctx, 0)
	require.NoError(t, err)
	require.False(t, migrationTableExists(t, ctx, sqlDB, cfg, "discoverability_content_audits"))
}

func TestM3B5DiscoverabilitySchemaUpDown(t *testing.T) {
	ctx, cfg, sqlDB := openMigrationTestDatabase(t)
	m3CleanupDiscoverabilitySchema(t, sqlDB)
	m3CreateDiscoverabilityParents(t, ctx, sqlDB, cfg)
	t.Cleanup(func() { m3CleanupDiscoverabilitySchema(t, sqlDB) })

	bundles := strictMigrationTestBundles(t, "discoverability")
	dialect := database.MigrationDialect(migrationTestDialect(t, cfg))
	provider := newMigrationTestProvider(t, sqlDB, cfg, bundles[dialect], "goose_discoverability_version")
	_, err := provider.UpTo(ctx, 2)
	require.NoError(t, err)
	for _, table := range []string{"discoverability_redirects", "discoverability_seo_defaults"} {
		require.True(t, migrationTableExists(t, ctx, sqlDB, cfg, table), table)
	}
	require.True(t, migrationHasExactIndexColumns(t, ctx, sqlDB, cfg, "discoverability_redirects", false, []string{"account_id", "updated_at", "id"}))
	_, err = provider.DownTo(ctx, 0)
	require.NoError(t, err)
	for _, table := range []string{"discoverability_seo_defaults", "discoverability_redirects"} {
		require.False(t, migrationTableExists(t, ctx, sqlDB, cfg, table), table)
	}
}

func m3CleanupDiscoverabilitySchema(t *testing.T, db *sql.DB) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	for _, table := range []string{"discoverability_content_audits", "discoverability_seo_defaults", "discoverability_redirects", "goose_discoverability_version", "content_translations", "content_items", "locales", "accounts"} {
		_, err := db.ExecContext(ctx, "DROP TABLE IF EXISTS "+table)
		require.NoError(t, err)
	}
}

func m3CreateDiscoverabilityAuditParents(t *testing.T, ctx context.Context, db *sql.DB, cfg config.DatabaseConfig) {
	t.Helper()
	idType := "UUID"
	if cfg.Driver == "mysql" {
		idType = "BINARY(16)"
	}
	_, err := db.ExecContext(ctx, "CREATE TABLE content_items (id "+idType+" PRIMARY KEY, account_id "+idType+" NOT NULL, UNIQUE (account_id, id))")
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, "CREATE TABLE content_translations (id "+idType+" PRIMARY KEY, account_id "+idType+" NOT NULL, UNIQUE (account_id, id))")
	require.NoError(t, err)
}

func m3CreateDiscoverabilityParents(t *testing.T, ctx context.Context, db *sql.DB, cfg config.DatabaseConfig) {
	t.Helper()
	idType := "UUID"
	if cfg.Driver == "mysql" {
		idType = "BINARY(16)"
	}
	_, err := db.ExecContext(ctx, "CREATE TABLE accounts (id "+idType+" PRIMARY KEY)")
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, "CREATE TABLE locales (id "+idType+" PRIMARY KEY)")
	require.NoError(t, err)
}
