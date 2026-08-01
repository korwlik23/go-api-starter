package migrations_test

import (
	"testing"

	"github.com/korwlik23/go-api-starter/internal/app"
	"github.com/stretchr/testify/require"
)

func TestM3A2PublishingMigrationModuleIsWired(t *testing.T) {
	t.Parallel()

	modules := app.MigrationModules()
	var publishingModuleID string
	for _, module := range modules {
		if module.ID == "publishing" {
			publishingModuleID = module.ID
			require.ElementsMatch(t, []string{"accounts", "localization"}, module.Dependencies)
		}
	}
	require.Equal(t, "publishing", publishingModuleID)
	require.NoError(t, app.ValidateMigrationDialectParity(modules))
}
