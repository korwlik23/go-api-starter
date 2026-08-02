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

func TestM3B4NavigationMigrationCatalog(t *testing.T) {
	t.Parallel()

	module := database.MigrationModule{ID: "navigation", Dependencies: []string{"accounts", "localization"}, Bundles: strictMigrationTestBundles(t, "navigation")}
	require.NoError(t, app.ValidateMigrationDialectParity([]database.MigrationModule{module}))
	expected := map[string][]string{
		"000001_create_navigation_menus.sql":      {"CREATE TABLE navigation_menus", "navigation_menus_account_locale_key_unique", "navigation_menus_locale_fk"},
		"000002_create_navigation_menu_items.sql": {"CREATE TABLE navigation_menu_items", "navigation_menu_items_menu_position_index", "navigation_menu_items_parent_fk"},
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

func TestM3B4NavigationSchemaUpDown(t *testing.T) {
	ctx, cfg, sqlDB := openMigrationTestDatabase(t)
	m3CleanupNavigationSchema(t, sqlDB)
	m3CreateNavigationParents(t, ctx, sqlDB, cfg)
	t.Cleanup(func() { m3CleanupNavigationSchema(t, sqlDB) })

	bundles := strictMigrationTestBundles(t, "navigation")
	dialect := database.MigrationDialect(migrationTestDialect(t, cfg))
	provider := newMigrationTestProvider(t, sqlDB, cfg, bundles[dialect], "goose_navigation_version")
	_, err := provider.Up(ctx)
	require.NoError(t, err)
	for _, table := range []string{"navigation_menus", "navigation_menu_items"} {
		require.True(t, migrationTableExists(t, ctx, sqlDB, cfg, table), table)
	}
	require.True(t, migrationHasExactIndexColumns(t, ctx, sqlDB, cfg, "navigation_menu_items", false, []string{"account_id", "menu_id", "parent_id", "position", "id"}))
	_, err = provider.DownTo(ctx, 0)
	require.NoError(t, err)
	for _, table := range []string{"navigation_menu_items", "navigation_menus"} {
		require.False(t, migrationTableExists(t, ctx, sqlDB, cfg, table), table)
	}
}

func m3CleanupNavigationSchema(t *testing.T, db *sql.DB) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	for _, table := range []string{"navigation_menu_items", "navigation_menus", "goose_navigation_version", "locales", "accounts"} {
		_, err := db.ExecContext(ctx, "DROP TABLE IF EXISTS "+table)
		require.NoError(t, err)
	}
}

func m3CreateNavigationParents(t *testing.T, ctx context.Context, db *sql.DB, cfg config.DatabaseConfig) {
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
