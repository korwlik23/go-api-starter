package migrations_test

import (
	"encoding/hex"
	"errors"
	"io/fs"
	"testing"
	"testing/fstest"

	"github.com/korwlik23/go-api-starter/internal/app"
	"github.com/korwlik23/go-api-starter/internal/platform/database"
	"github.com/stretchr/testify/require"
)

func TestD4MigrationParityAcceptsDialectSpecificSQLWithMatchingNames(
	t *testing.T,
) {
	t.Parallel()

	modules := []database.MigrationModule{
		d4MigrationModule(
			"localization",
			map[string]string{
				"000001_create_locales.sql":      "postgres locales",
				"000003_create_translations.sql": "postgres translations",
			},
			map[string]string{
				"000001_create_locales.sql":      "mariadb locales",
				"000003_create_translations.sql": "mariadb translations",
			},
			map[string]string{
				"000001_create_locales.sql":      "mysql locales",
				"000003_create_translations.sql": "mysql translations",
			},
		),
		d4MigrationModule(
			"identity",
			map[string]string{"000001_create_users.sql": "postgres users"},
			map[string]string{"000001_create_users.sql": "mariadb users"},
			map[string]string{"000001_create_users.sql": "mysql users"},
		),
	}

	require.NoError(t, app.ValidateMigrationDialectParity(modules))
}

func TestD4MigrationCatalogChecksumIsDeterministicAndByteSensitive(
	t *testing.T,
) {
	t.Parallel()

	identity := func(mysqlSQL string) database.MigrationModule {
		return d4MigrationModule(
			"identity",
			map[string]string{"000001_create_users.sql": "postgres users"},
			map[string]string{"000001_create_users.sql": "mariadb users"},
			map[string]string{"000001_create_users.sql": mysqlSQL},
		)
	}
	accounts := func() database.MigrationModule {
		return d4MigrationModule(
			"accounts",
			map[string]string{"000001_create_accounts.sql": "postgres accounts"},
			map[string]string{"000001_create_accounts.sql": "mariadb accounts"},
			map[string]string{"000001_create_accounts.sql": "mysql accounts"},
		)
	}

	firstChecksum, err := app.MigrationCatalogChecksum(
		[]database.MigrationModule{
			identity("mysql users"),
			accounts(),
		},
	)
	require.NoError(t, err)
	secondChecksum, err := app.MigrationCatalogChecksum(
		[]database.MigrationModule{
			accounts(),
			identity("mysql users"),
		},
	)
	require.NoError(t, err)
	mutatedChecksum, err := app.MigrationCatalogChecksum(
		[]database.MigrationModule{
			accounts(),
			identity("mysql users!"),
		},
	)
	require.NoError(t, err)

	require.Equal(t, firstChecksum, secondChecksum)
	require.NotEqual(t, firstChecksum, mutatedChecksum)
	require.Equal(
		t,
		"1b5af29ff0f97007e9e5e7109979ef1aa71788b6cbb50f4e05f8cafe25555aff",
		firstChecksum,
	)
	require.Len(t, firstChecksum, 64)
	_, err = hex.DecodeString(firstChecksum)
	require.NoError(t, err)
}

func TestD4MigrationParityRejectsDialectDriftWithLogicalPath(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		module       database.MigrationModule
		expectedPath string
	}{
		{
			name: "missing migration",
			module: d4MigrationModule(
				"identity",
				map[string]string{
					"000001_create_users.sql":    "postgres users",
					"000002_create_sessions.sql": "postgres sessions",
				},
				map[string]string{
					"000001_create_users.sql":    "mariadb users",
					"000002_create_sessions.sql": "mariadb sessions",
				},
				map[string]string{
					"000001_create_users.sql": "mysql users",
				},
			),
			expectedPath: "internal/modules/identity/migrations/mysql/" +
				"000002_create_sessions.sql",
		},
		{
			name: "basename drift",
			module: d4MigrationModule(
				"identity",
				map[string]string{
					"000001_create_users.sql": "postgres users",
				},
				map[string]string{
					"000001_create_users.sql": "mariadb users",
				},
				map[string]string{
					"000001_create_user.sql": "mysql user",
				},
			),
			expectedPath: "internal/modules/identity/migrations/mysql/" +
				"000001_create_user.sql",
		},
		{
			name: "extra migration",
			module: d4MigrationModule(
				"identity",
				map[string]string{
					"000001_create_users.sql": "postgres users",
				},
				map[string]string{
					"000001_create_users.sql": "mariadb users",
				},
				map[string]string{
					"000001_create_users.sql":    "mysql users",
					"000002_create_sessions.sql": "mysql sessions",
				},
			),
			expectedPath: "internal/modules/identity/migrations/mysql/" +
				"000002_create_sessions.sql",
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			err := app.ValidateMigrationDialectParity(
				[]database.MigrationModule{test.module},
			)

			require.ErrorIs(t, err, app.ErrInvalidMigrationCatalog)
			require.Contains(t, err.Error(), test.expectedPath)
		})
	}
}

func TestD4MigrationParityRejectsInvalidBundleEntries(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		postgres     fs.FS
		expectedPath string
	}{
		{
			name: "duplicate version",
			postgres: d4MigrationFS(map[string]string{
				"000001_create_account.sql":  "first",
				"000001_create_accounts.sql": "duplicate",
			}),
			expectedPath: "internal/modules/accounts/migrations/postgres/" +
				"000001_create_accounts.sql",
		},
		{
			name: "zero version",
			postgres: d4MigrationFS(map[string]string{
				"000000_create_accounts.sql": "zero",
			}),
			expectedPath: "internal/modules/accounts/migrations/postgres/" +
				"000000_create_accounts.sql",
		},
		{
			name: "unpadded version",
			postgres: d4MigrationFS(map[string]string{
				"1_create_accounts.sql": "unpadded",
			}),
			expectedPath: "internal/modules/accounts/migrations/postgres/" +
				"1_create_accounts.sql",
		},
		{
			name: "non canonical description",
			postgres: d4MigrationFS(map[string]string{
				"000001_CreateAccounts.sql": "uppercase",
			}),
			expectedPath: "internal/modules/accounts/migrations/postgres/" +
				"000001_CreateAccounts.sql",
		},
		{
			name: "non SQL file",
			postgres: d4MigrationFS(map[string]string{
				"README.md": "ignored by Goose",
			}),
			expectedPath: "internal/modules/accounts/migrations/postgres/" +
				"README.md",
		},
		{
			name: "nested directory",
			postgres: d4MigrationFS(map[string]string{
				"archive/000001_create_accounts.sql": "nested",
			}),
			expectedPath: "internal/modules/accounts/migrations/postgres/" +
				"archive",
		},
		{
			name: "empty migration file",
			postgres: d4MigrationFS(map[string]string{
				"000001_create_accounts.sql": "",
			}),
			expectedPath: "internal/modules/accounts/migrations/postgres/" +
				"000001_create_accounts.sql",
		},
		{
			name: "unknown entry type resolves to non regular file",
			postgres: d4UnknownTypeFS{
				files: d4MigrationFS(map[string]string{
					"000001_create_accounts.sql": "symlink-like file",
				}),
				name: "000001_create_accounts.sql",
				mode: fs.ModeSymlink,
			},
			expectedPath: "internal/modules/accounts/migrations/postgres/" +
				"000001_create_accounts.sql",
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			module := d4MigrationModule(
				"accounts",
				map[string]string{"000001_create_accounts.sql": "postgres"},
				map[string]string{"000001_create_accounts.sql": "mariadb"},
				map[string]string{"000001_create_accounts.sql": "mysql"},
			)
			module.Bundles[database.MigrationDialectPostgres] = test.postgres

			err := app.ValidateMigrationDialectParity(
				[]database.MigrationModule{module},
			)

			require.ErrorIs(t, err, app.ErrInvalidMigrationCatalog)
			require.Contains(t, err.Error(), test.expectedPath)
		})
	}
}

func TestD4MigrationParityRejectsIncompleteOrUnsafePlans(t *testing.T) {
	t.Parallel()

	validModule := func() database.MigrationModule {
		return d4MigrationModule(
			"identity",
			map[string]string{"000001_create_users.sql": "postgres"},
			map[string]string{"000001_create_users.sql": "mariadb"},
			map[string]string{"000001_create_users.sql": "mysql"},
		)
	}
	tests := []struct {
		name              string
		modules           []database.MigrationModule
		expectedPath      string
		privateErrorValue string
	}{
		{
			name:    "empty module list",
			modules: nil,
		},
		{
			name: "duplicate module",
			modules: []database.MigrationModule{
				validModule(),
				validModule(),
			},
			expectedPath: "internal/modules/identity/migrations",
		},
		{
			name: "missing dialect bundle",
			modules: func() []database.MigrationModule {
				module := validModule()
				delete(
					module.Bundles,
					database.MigrationDialectMariaDB,
				)
				return []database.MigrationModule{module}
			}(),
			expectedPath: "internal/modules/identity/migrations/mariadb",
		},
		{
			name: "empty dialect bundle",
			modules: func() []database.MigrationModule {
				module := validModule()
				module.Bundles[database.MigrationDialectMySQL] = fstest.MapFS{}
				return []database.MigrationModule{module}
			}(),
			expectedPath: "internal/modules/identity/migrations/mysql",
		},
		{
			name: "typed nil dialect bundle",
			modules: func() []database.MigrationModule {
				module := validModule()
				var nilBundle fstest.MapFS
				module.Bundles[database.MigrationDialectMySQL] = nilBundle
				return []database.MigrationModule{module}
			}(),
			expectedPath: "internal/modules/identity/migrations/mysql",
		},
		{
			name: "unsupported dialect bundle",
			modules: func() []database.MigrationModule {
				module := validModule()
				module.Bundles[database.MigrationDialect("sqlite")] =
					d4MigrationFS(
						map[string]string{"000001_create_users.sql": "sqlite"},
					)
				return []database.MigrationModule{module}
			}(),
			expectedPath: "internal/modules/identity/migrations",
		},
		{
			name: "unsafe module identifier",
			modules: func() []database.MigrationModule {
				module := validModule()
				module.ID = "../../private-module-marker"
				return []database.MigrationModule{module}
			}(),
			privateErrorValue: "private-module-marker",
		},
		{
			name: "filesystem read failure",
			modules: func() []database.MigrationModule {
				module := validModule()
				module.Bundles[database.MigrationDialectPostgres] =
					d4ErrorFS{err: errors.New("private-filesystem-marker")}
				return []database.MigrationModule{module}
			}(),
			expectedPath:      "internal/modules/identity/migrations/postgres",
			privateErrorValue: "private-filesystem-marker",
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			err := app.ValidateMigrationDialectParity(test.modules)

			require.ErrorIs(t, err, app.ErrInvalidMigrationCatalog)
			if test.expectedPath != "" {
				require.Contains(t, err.Error(), test.expectedPath)
			}
			if test.privateErrorValue != "" {
				require.NotContains(t, err.Error(), test.privateErrorValue)
			}
		})
	}
}
