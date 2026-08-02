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

func TestM3B2MediaMigrationCatalog(t *testing.T) {
	t.Parallel()

	module := database.MigrationModule{ID: "media", Dependencies: []string{"accounts"}, Bundles: strictMigrationTestBundles(t, "media")}
	require.NoError(t, app.ValidateMigrationDialectParity([]database.MigrationModule{module}))
	expected := map[string][]string{
		"000001_create_media_assets.sql":  {"CREATE TABLE media_assets", "media_assets_account_updated_id_index", "media_assets_account_fk"},
		"000002_create_media_uploads.sql": {"CREATE TABLE media_uploads", "media_uploads_account_state_expiry_index", "media_uploads_asset_fk"},
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

func TestM3B2MediaSchemaUpDown(t *testing.T) {
	ctx, cfg, sqlDB := openMigrationTestDatabase(t)
	m3CleanupMediaSchema(t, sqlDB)
	m3CreateMediaParents(t, ctx, sqlDB, cfg)
	t.Cleanup(func() { m3CleanupMediaSchema(t, sqlDB) })

	provider := newMigrationTestProvider(t, sqlDB, cfg, strictMigrationTestBundles(t, "media")[database.MigrationDialect(migrationTestDialect(t, cfg))], "goose_media_version")
	_, err := provider.Up(ctx)
	require.NoError(t, err)
	for _, table := range []string{"media_assets", "media_uploads"} {
		require.True(t, migrationTableExists(t, ctx, sqlDB, cfg, table), table)
	}
	require.True(t, migrationHasExactIndexColumns(t, ctx, sqlDB, cfg, "media_assets", false, []string{"account_id", "updated_at", "id"}))
	_, err = provider.DownTo(ctx, 0)
	require.NoError(t, err)
	for _, table := range []string{"media_uploads", "media_assets"} {
		require.False(t, migrationTableExists(t, ctx, sqlDB, cfg, table), table)
	}
}

func m3CleanupMediaSchema(t *testing.T, db *sql.DB) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	for _, table := range []string{"media_uploads", "media_assets", "goose_media_version", "accounts"} {
		_, err := db.ExecContext(ctx, "DROP TABLE IF EXISTS "+table)
		require.NoError(t, err)
	}
}

func m3CreateMediaParents(t *testing.T, ctx context.Context, db *sql.DB, cfg config.DatabaseConfig) {
	t.Helper()
	idType := "UUID"
	if cfg.Driver == "mysql" {
		idType = "BINARY(16)"
	}
	_, err := db.ExecContext(ctx, "CREATE TABLE accounts (id "+idType+" PRIMARY KEY)")
	require.NoError(t, err)
}
