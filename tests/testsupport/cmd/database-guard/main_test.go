package main

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestD0_3(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		environment   map[string]string
		expectedCode  int
		expectedError string
	}{
		{
			name: "allows an explicitly authorized disposable database",
			environment: map[string]string{
				"APP_ENV":                   "test",
				"DB_NAME":                   "go_lang_starter_test_postgres",
				"DB_PASSWORD":               "private-password-marker",
				"ALLOW_TEST_DATABASE_RESET": "1",
			},
		},
		{
			name: "rejects a non test environment",
			environment: map[string]string{
				"APP_ENV":                   "development",
				"DB_NAME":                   "go_lang_starter_test_postgres",
				"DB_PASSWORD":               "private-password-marker",
				"ALLOW_TEST_DATABASE_RESET": "1",
			},
			expectedCode:  1,
			expectedError: "testsupport: destructive database operation requires APP_ENV=test\n",
		},
		{
			name: "rejects an unsafe database without exposing its name",
			environment: map[string]string{
				"APP_ENV":                   "test",
				"DB_NAME":                   "private-application-database",
				"DB_PASSWORD":               "private-password-marker",
				"ALLOW_TEST_DATABASE_RESET": "1",
			},
			expectedCode: 1,
			expectedError: "testsupport: destructive database operation requires " +
				"DB_NAME prefix go_lang_starter_test_\n",
		},
		{
			name: "rejects authorization without exposing its value",
			environment: map[string]string{
				"APP_ENV":                   "test",
				"DB_NAME":                   "go_lang_starter_test_mariadb_xampp",
				"DB_PASSWORD":               "private-password-marker",
				"ALLOW_TEST_DATABASE_RESET": "private-authorization-marker",
			},
			expectedCode: 1,
			expectedError: "testsupport: destructive database operation requires " +
				"ALLOW_TEST_DATABASE_RESET=1\n",
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			var stderr bytes.Buffer
			code := run(mapLookup(test.environment), &stderr)

			require.Equal(t, test.expectedCode, code)
			require.Equal(t, test.expectedError, stderr.String())
			require.NotContains(t, stderr.String(), test.environment["DB_PASSWORD"])
			if test.expectedCode != 0 {
				require.NotContains(t, stderr.String(), test.environment["DB_NAME"])
				require.NotContains(
					t,
					stderr.String(),
					test.environment["ALLOW_TEST_DATABASE_RESET"],
				)
			}
		})
	}
}

func mapLookup(values map[string]string) func(string) (string, bool) {
	return func(key string) (string, bool) {
		value, exists := values[key]
		return value, exists
	}
}
