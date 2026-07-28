package migrations_test

import (
	"database/sql"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"testing/fstest"

	"github.com/korwlik23/go-api-starter/internal/platform/config"
	"github.com/korwlik23/go-api-starter/internal/platform/database"
	"github.com/pressly/goose/v3"
	"github.com/stretchr/testify/require"
)

func newMigrationTestProvider(
	t *testing.T,
	db *sql.DB,
	cfg config.DatabaseConfig,
	bundle fs.FS,
	versionTable string,
) *goose.Provider {
	t.Helper()

	provider, err := goose.NewProvider(
		migrationTestGooseDialect(t, cfg),
		db,
		bundle,
		goose.WithTableName(versionTable),
		goose.WithDisableGlobalRegistry(true),
		goose.WithLogger(goose.NopLogger()),
	)
	require.NoError(t, err)
	return provider
}

func migrationTestFileBundle(
	t *testing.T,
	module string,
	dialect string,
	filename string,
	allowMissingPlaceholder bool,
) fs.FS {
	t.Helper()

	content, err := os.ReadFile(filepath.Join(
		migrationTestBundlePath(t, module, dialect),
		filename,
	))
	if errors.Is(err, os.ErrNotExist) && allowMissingPlaceholder {
		content = []byte(
			"-- +goose Up\nSELECT 1;\n" +
				"-- +goose Down\nSELECT 1;\n",
		)
	} else {
		require.NoError(t, err)
	}
	return fstest.MapFS{filename: &fstest.MapFile{Data: content}}
}

func strictMigrationTestBundles(
	t *testing.T,
	module string,
) map[database.MigrationDialect]fs.FS {
	t.Helper()

	return map[database.MigrationDialect]fs.FS{
		database.MigrationDialectPostgres: os.DirFS(
			migrationTestBundlePath(t, module, "postgres"),
		),
		database.MigrationDialectMariaDB: os.DirFS(
			migrationTestBundlePath(t, module, "mariadb"),
		),
		database.MigrationDialectMySQL: os.DirFS(
			migrationTestBundlePath(t, module, "mysql"),
		),
	}
}

func migrationTestBundlePath(t *testing.T, module, dialect string) string {
	t.Helper()

	_, filename, _, ok := runtime.Caller(0)
	require.True(t, ok)
	return filepath.Join(
		filepath.Dir(filename),
		"..",
		"..",
		"internal",
		"modules",
		module,
		"migrations",
		dialect,
	)
}

func migrationTestDialect(t *testing.T, cfg config.DatabaseConfig) string {
	t.Helper()

	switch {
	case cfg.Driver == "postgres" && cfg.Flavor == "":
		return "postgres"
	case cfg.Driver == "mysql" && cfg.Flavor == "mariadb":
		return "mariadb"
	case cfg.Driver == "mysql" && cfg.Flavor == "mysql":
		return "mysql"
	default:
		t.Fatalf("unsupported migration database profile")
		return ""
	}
}

func migrationTestGooseDialect(
	t *testing.T,
	cfg config.DatabaseConfig,
) goose.Dialect {
	t.Helper()

	if cfg.Driver == "postgres" && cfg.Flavor == "" {
		return goose.DialectPostgres
	}
	if cfg.Driver == "mysql" && (cfg.Flavor == "mariadb" || cfg.Flavor == "mysql") {
		return goose.DialectMySQL
	}
	t.Fatalf("unsupported migration database profile")
	return ""
}
