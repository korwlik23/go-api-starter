package testsupport

import (
	"testing"

	"github.com/korwlik23/go-api-starter/internal/platform/config"
	"github.com/stretchr/testify/require"
)

func TestD0_2(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name               string
		environment        config.Environment
		databaseName       string
		resetAuthorization string
		expectedError      string
	}{
		{
			name:               "allows a disposable PostgreSQL database",
			environment:        config.Environment("test"),
			databaseName:       "go_lang_starter_test_postgres",
			resetAuthorization: "1",
		},
		{
			name:               "allows a dedicated MariaDB XAMPP database",
			environment:        config.Environment("test"),
			databaseName:       "go_lang_starter_test_mariadb_xampp",
			resetAuthorization: "1",
		},
		{
			name:          "rejects zero config",
			expectedError: "testsupport: destructive database operation requires APP_ENV=test",
		},
		{
			name:         "rejects development",
			environment:  config.Environment("development"),
			databaseName: "go_lang_starter_dev",
			expectedError: "testsupport: destructive database operation requires " +
				"APP_ENV=test",
		},
		{
			name:               "rejects production even with a test database name",
			environment:        config.Environment("production"),
			databaseName:       "go_lang_starter_test_postgres",
			resetAuthorization: "1",
			expectedError: "testsupport: destructive database operation requires " +
				"APP_ENV=test",
		},
		{
			name:               "rejects non canonical test environment",
			environment:        config.Environment("TEST"),
			databaseName:       "go_lang_starter_test_postgres",
			resetAuthorization: "1",
			expectedError: "testsupport: destructive database operation requires " +
				"APP_ENV=test",
		},
		{
			name:               "rejects an application database",
			environment:        config.Environment("test"),
			databaseName:       "go_lang_starter_dev",
			resetAuthorization: "1",
			expectedError: "testsupport: destructive database operation requires " +
				"DB_NAME prefix go_lang_starter_test_",
		},
		{
			name:               "rejects an existing XAMPP application database",
			environment:        config.Environment("test"),
			databaseName:       "private-xampp-application-database",
			resetAuthorization: "1",
			expectedError: "testsupport: destructive database operation requires " +
				"DB_NAME prefix go_lang_starter_test_",
		},
		{
			name:               "rejects a case variant database prefix",
			environment:        config.Environment("test"),
			databaseName:       "GO_LANG_STARTER_TEST_xampp",
			resetAuthorization: "1",
			expectedError: "testsupport: destructive database operation requires " +
				"DB_NAME prefix go_lang_starter_test_",
		},
		{
			name:         "rejects missing reset authorization",
			environment:  config.Environment("test"),
			databaseName: "go_lang_starter_test_postgres",
			expectedError: "testsupport: destructive database operation requires " +
				"ALLOW_TEST_DATABASE_RESET=1",
		},
		{
			name:               "rejects boolean reset authorization",
			environment:        config.Environment("test"),
			databaseName:       "go_lang_starter_test_postgres",
			resetAuthorization: "true",
			expectedError: "testsupport: destructive database operation requires " +
				"ALLOW_TEST_DATABASE_RESET=1",
		},
		{
			name:               "rejects padded reset authorization",
			environment:        config.Environment("test"),
			databaseName:       "go_lang_starter_test_postgres",
			resetAuthorization: " 1 ",
			expectedError: "testsupport: destructive database operation requires " +
				"ALLOW_TEST_DATABASE_RESET=1",
		},
		{
			name:               "rejects alternate numeric reset authorization",
			environment:        config.Environment("test"),
			databaseName:       "go_lang_starter_test_postgres",
			resetAuthorization: "01",
			expectedError: "testsupport: destructive database operation requires " +
				"ALLOW_TEST_DATABASE_RESET=1",
		},
		{
			name:               "does not expose rejected reset authorization",
			environment:        config.Environment("test"),
			databaseName:       "go_lang_starter_test_postgres",
			resetAuthorization: "private-reset-authorization",
			expectedError: "testsupport: destructive database operation requires " +
				"ALLOW_TEST_DATABASE_RESET=1",
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			cfg := config.Config{
				App: config.AppConfig{
					Environment: test.environment,
				},
				Database: config.DatabaseConfig{
					Name:     test.databaseName,
					Password: "private-database-password",
				},
			}

			err := RequireDisposableDatabase(cfg, test.resetAuthorization)
			if test.expectedError == "" {
				require.NoError(t, err)
				return
			}

			require.EqualError(t, err, test.expectedError)
			require.NotContains(t, err.Error(), cfg.Database.Password)
			if test.databaseName != "" {
				require.NotContains(t, err.Error(), test.databaseName)
			}
			if test.resetAuthorization != "" {
				require.NotContains(t, err.Error(), test.resetAuthorization)
			}
		})
	}
}
