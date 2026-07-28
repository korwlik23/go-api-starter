package main

import (
	"fmt"
	"io"
	"os"

	"github.com/korwlik23/go-api-starter/internal/platform/config"
	"github.com/korwlik23/go-api-starter/tests/testsupport"
)

type lookupFunc func(string) (string, bool)

func main() {
	os.Exit(run(os.LookupEnv, os.Stderr))
}

func run(lookup lookupFunc, stderr io.Writer) int {
	environment, _ := lookup("APP_ENV")
	databaseName, _ := lookup("DB_NAME")
	resetAuthorization, _ := lookup("ALLOW_TEST_DATABASE_RESET")

	cfg := config.Config{
		App: config.AppConfig{
			Environment: config.Environment(environment),
		},
		Database: config.DatabaseConfig{
			Name: databaseName,
		},
	}
	if err := testsupport.RequireDisposableDatabase(
		cfg,
		resetAuthorization,
	); err != nil {
		_, _ = fmt.Fprintln(stderr, err)
		return 1
	}
	return 0
}
