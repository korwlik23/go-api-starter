package publishing

import (
	"embed"
	"io/fs"

	"github.com/korwlik23/go-api-starter/internal/platform/database"
)

//go:embed migrations/postgres/*.sql migrations/mariadb/*.sql migrations/mysql/*.sql
var migrationFiles embed.FS

func MigrationModule() database.MigrationModule {
	return database.MigrationModule{
		ID:           "publishing",
		Dependencies: []string{"accounts", "identity", "localization"},
		Bundles:      migrationBundles(migrationFiles),
	}
}

func migrationBundles(files embed.FS) map[database.MigrationDialect]fs.FS {
	bundles := make(map[database.MigrationDialect]fs.FS, 3)
	for _, dialect := range []database.MigrationDialect{
		database.MigrationDialectPostgres,
		database.MigrationDialectMariaDB,
		database.MigrationDialectMySQL,
	} {
		bundle, err := fs.Sub(files, "migrations/"+string(dialect))
		if err != nil {
			panic("publishing migration bundle is invalid")
		}
		bundles[dialect] = bundle
	}
	return bundles
}
