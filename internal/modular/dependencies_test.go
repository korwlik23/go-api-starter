package modular

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRegistryOrdersDependenciesBeforeDependents(t *testing.T) {
	t.Parallel()

	registry, err := NewRegistry([]Manifest{
		registryManifest("audit", "accounts", "identity"),
		registryManifest("identity"),
		registryManifest("accounts", "identity"),
	})

	require.NoError(t, err)
	ordered := registry.Ordered()
	require.Equal(t, []string{"identity", "accounts", "audit"}, manifestIDs(ordered))
}

func TestRegistryRejectsInvalidDependencyGraph(t *testing.T) {
	t.Parallel()

	tests := map[string][]Manifest{
		"duplicate module": {
			registryManifest("identity"),
			registryManifest("identity"),
		},
		"missing dependency": {
			registryManifest("accounts", "identity"),
		},
		"cycle": {
			registryManifest("identity", "accounts"),
			registryManifest("accounts", "identity"),
		},
	}

	for name, manifests := range tests {
		name, manifests := name, manifests
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			_, err := NewRegistry(manifests)
			require.Error(t, err)
		})
	}
}

func registryManifest(id string, dependencies ...string) Manifest {
	return Manifest{
		ID:           id,
		Tier:         TierCore,
		Dependencies: dependencies,
		Migrations: []Migration{
			{Dialect: "postgres"},
			{Dialect: "mariadb"},
			{Dialect: "mysql"},
		},
	}
}

func manifestIDs(manifests []Manifest) []string {
	ids := make([]string, 0, len(manifests))
	for _, manifest := range manifests {
		ids = append(ids, manifest.ID)
	}
	return ids
}
