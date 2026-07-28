package migrations_test

import (
	"io/fs"
	"testing/fstest"
	"time"

	"github.com/korwlik23/go-api-starter/internal/platform/database"
)

func d4MigrationModule(
	id string,
	postgres map[string]string,
	mariadb map[string]string,
	mysql map[string]string,
) database.MigrationModule {
	return database.MigrationModule{
		ID: id,
		Bundles: map[database.MigrationDialect]fs.FS{
			database.MigrationDialectPostgres: d4MigrationFS(postgres),
			database.MigrationDialectMariaDB:  d4MigrationFS(mariadb),
			database.MigrationDialectMySQL:    d4MigrationFS(mysql),
		},
	}
}

func d4MigrationFS(files map[string]string) fstest.MapFS {
	migrationFS := make(fstest.MapFS, len(files))
	for name, content := range files {
		migrationFS[name] = &fstest.MapFile{Data: []byte(content)}
	}
	return migrationFS
}

type d4ErrorFS struct {
	err error
}

func (filesystem d4ErrorFS) Open(string) (fs.File, error) {
	return nil, filesystem.err
}

type d4UnknownTypeFS struct {
	files fstest.MapFS
	name  string
	mode  fs.FileMode
}

func (filesystem d4UnknownTypeFS) Open(name string) (fs.File, error) {
	return filesystem.files.Open(name)
}

func (filesystem d4UnknownTypeFS) ReadDir(
	string,
) ([]fs.DirEntry, error) {
	return []fs.DirEntry{
		d4UnknownTypeDirEntry{
			name: filesystem.name,
			mode: filesystem.mode,
		},
	}, nil
}

type d4UnknownTypeDirEntry struct {
	name string
	mode fs.FileMode
}

func (entry d4UnknownTypeDirEntry) Name() string {
	return entry.name
}

func (d4UnknownTypeDirEntry) IsDir() bool {
	return false
}

func (d4UnknownTypeDirEntry) Type() fs.FileMode {
	return 0
}

func (entry d4UnknownTypeDirEntry) Info() (fs.FileInfo, error) {
	return d4FileInfo{name: entry.name, mode: entry.mode}, nil
}

type d4FileInfo struct {
	name string
	mode fs.FileMode
}

func (info d4FileInfo) Name() string {
	return info.name
}

func (d4FileInfo) Size() int64 {
	return 0
}

func (info d4FileInfo) Mode() fs.FileMode {
	return info.mode
}

func (d4FileInfo) ModTime() time.Time {
	return time.Time{}
}

func (info d4FileInfo) IsDir() bool {
	return info.mode.IsDir()
}

func (d4FileInfo) Sys() any {
	return nil
}
