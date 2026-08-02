package app

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMigrationModulesEmbedValidDialectParity(t *testing.T) {
	t.Parallel()

	modules := MigrationModules()
	require.Len(t, modules, 8)
	require.NoError(t, ValidateMigrationDialectParity(modules))
	checksum, err := MigrationCatalogChecksum(modules)
	require.NoError(t, err)
	require.Len(t, checksum, 64)
}
