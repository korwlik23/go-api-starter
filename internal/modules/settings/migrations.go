package settings

import (
	"embed"
	"io/fs"

	"github.com/korwlik23/go-api-starter/internal/platform/database"
)

//go:embed migrations/postgres/*.sql migrations/mariadb/*.sql migrations/mysql/*.sql
var migrationFiles embed.FS

func MigrationModule() database.MigrationModule {
	return database.MigrationModule{ID: "settings", Bundles: migrationBundles(migrationFiles)}
}

func migrationBundles(files embed.FS) map[database.MigrationDialect]fs.FS {
	bundles := make(map[database.MigrationDialect]fs.FS, 3)
	for _, dialect := range []database.MigrationDialect{database.MigrationDialectPostgres, database.MigrationDialectMariaDB, database.MigrationDialectMySQL} {
		bundle, err := fs.Sub(files, "migrations/"+string(dialect))
		if err != nil {
			panic("settings migration bundle is invalid")
		}
		bundles[dialect] = bundle
	}
	return bundles
}
