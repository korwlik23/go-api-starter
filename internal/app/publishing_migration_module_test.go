package app

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestM3A2PublishingMigrationModuleIsRegistered(t *testing.T) {
	t.Parallel()

	modules := MigrationModules()
	require.Len(t, modules, 12)
	var found bool
	for _, module := range modules {
		if module.ID != "publishing" {
			continue
		}
		found = true
		require.ElementsMatch(t, []string{"accounts", "identity", "localization"}, module.Dependencies)
		require.NotNil(t, module.Bundles)
	}
	require.True(t, found)
}
