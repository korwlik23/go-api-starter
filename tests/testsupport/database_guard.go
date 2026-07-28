package testsupport

import (
	"errors"
	"strings"

	"github.com/korwlik23/go-api-starter/internal/platform/config"
)

const disposableDatabasePrefix = "go_lang_starter_test_"

var (
	errTestEnvironmentRequired = errors.New(
		"testsupport: destructive database operation requires APP_ENV=test",
	)
	errDisposableDatabaseNameRequired = errors.New(
		"testsupport: destructive database operation requires " +
			"DB_NAME prefix go_lang_starter_test_",
	)
	errDatabaseResetAuthorizationRequired = errors.New(
		"testsupport: destructive database operation requires " +
			"ALLOW_TEST_DATABASE_RESET=1",
	)
)

// RequireDisposableDatabase rejects destructive test-database operations
// unless every independent safety latch exactly matches the approved contract.
func RequireDisposableDatabase(
	cfg config.Config,
	resetAuthorization string,
) error {
	if cfg.App.Environment != config.Environment("test") {
		return errTestEnvironmentRequired
	}
	if !strings.HasPrefix(cfg.Database.Name, disposableDatabasePrefix) {
		return errDisposableDatabaseNameRequired
	}
	if resetAuthorization != "1" {
		return errDatabaseResetAuthorizationRequired
	}
	return nil
}
