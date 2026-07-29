package modular

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestProjectStateRemovesDisabledRuntimeSurface(t *testing.T) {
	t.Parallel()

	optional := validManifestFixture()
	optional.ID = "publishing"
	optional.Tier = TierOptional
	optional.Disableable = true
	optional.Dependencies = []string{"identity"}
	optional.Permissions[0] = Permission{
		Key:       "publishing.posts.read.any",
		Scope:     ScopeAny,
		Tier:      TierOptional,
		Delegable: true,
	}
	optional.Routes[0].Permission = optional.Permissions[0].Key

	registry, err := NewRegistry([]Manifest{
		registryManifest("identity"),
		optional,
	})
	require.NoError(t, err)

	projection, err := ProjectState(registry, map[string]bool{
		"identity":   true,
		"publishing": false,
	})
	require.NoError(t, err)

	publishing, ok := projection.Module("publishing")
	require.True(t, ok)
	require.False(t, publishing.Enabled)
	require.Empty(t, publishing.Routes)
	require.Empty(t, publishing.Jobs)
	require.Empty(t, publishing.Health)
	require.Len(t, publishing.Permissions, 1)
	require.False(t, publishing.Permissions[0].Active)
}

func TestProjectStateRejectsDisabledCoreOrDependency(t *testing.T) {
	t.Parallel()

	optional := registryManifest("publishing", "identity")
	optional.Tier = TierOptional
	optional.Disableable = true
	registry, err := NewRegistry([]Manifest{
		registryManifest("identity"),
		optional,
	})
	require.NoError(t, err)

	_, err = ProjectState(registry, map[string]bool{
		"identity":   false,
		"publishing": false,
	})
	require.Error(t, err)

	_, err = ProjectState(registry, map[string]bool{
		"identity":   false,
		"publishing": true,
	})
	require.Error(t, err)
}
