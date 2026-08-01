package app

import (
	"testing"

	"github.com/korwlik23/go-api-starter/internal/modular"
	"github.com/stretchr/testify/require"
)

func TestM3A1PublishingModuleIsRegisteredWithDefaultContract(t *testing.T) {
	t.Parallel()

	registry, err := Modules()
	require.NoError(t, err)

	manifest, exists := registry.Manifest("publishing")
	require.True(t, exists)
	require.Equal(t, modular.TierDefault, manifest.Tier)
	require.True(t, manifest.Disableable)
	require.ElementsMatch(
		t,
		[]string{"accounts", "authorization", "localization", "audit"},
		manifest.Dependencies,
	)
	require.ElementsMatch(
		t,
		[]string{
			"accounts.resolve",
			"authorization.evaluate",
			"audit.append",
			"database.sql",
			"localization.catalog",
		},
		manifest.RequiredCapabilities,
	)
	require.Contains(t, manifest.ProvidedCapabilities, "publishing.content")
	require.ElementsMatch(
		t,
		[]modular.Migration{
			{Dialect: "postgres"},
			{Dialect: "mariadb"},
			{Dialect: "mysql"},
		},
		manifest.Migrations,
	)
	require.Contains(
		t,
		manifest.Routes,
		modular.Route{
			Method:     "GET",
			Path:       "/api/v1/public/content/:locale/*path",
			Permission: "",
		},
	)
	require.Contains(t, manifest.Jobs, modular.Job{ID: "publishing.schedule.process"})
	require.Contains(
		t,
		manifest.Health,
		modular.HealthContributor{ID: "publishing.content", Critical: true},
	)
}

func TestM3A1PublishingPermissionsAreCatalogued(t *testing.T) {
	t.Parallel()

	registry, err := Modules()
	require.NoError(t, err)

	for _, key := range []string{
		"publishing.pages.read.own",
		"publishing.pages.write.own",
		"publishing.pages.review.any",
		"publishing.pages.publish.any",
		"publishing.revisions.read.own",
		"publishing.revisions.rollback.own",
	} {
		permission, exists := registry.Permission(key)
		require.True(t, exists, key)
		require.Equal(t, modular.TierDefault, permission.Tier, key)
	}
}

func TestM3A1PublishingModuleFollowsItsDependencies(t *testing.T) {
	t.Parallel()

	registry, err := Modules()
	require.NoError(t, err)

	positions := make(map[string]int)
	for index, manifest := range registry.Ordered() {
		positions[manifest.ID] = index
	}
	publishingPosition, exists := positions["publishing"]
	require.True(t, exists)
	for _, dependency := range []string{"accounts", "authorization", "localization", "audit"} {
		dependencyPosition, exists := positions[dependency]
		require.True(t, exists, dependency)
		require.Less(t, dependencyPosition, publishingPosition, dependency)
	}
}

func TestM3A1PublishingModuleCanBeDisabledWithoutRuntimeSurface(t *testing.T) {
	t.Parallel()

	registry, err := Modules()
	require.NoError(t, err)
	enabled := make(map[string]bool)
	for _, manifest := range registry.Ordered() {
		enabled[manifest.ID] = manifest.ID != "publishing"
	}

	projection, err := modular.ProjectState(registry, enabled)
	require.NoError(t, err)
	publishing, exists := projection.Module("publishing")
	require.True(t, exists)
	require.False(t, publishing.Enabled)
	require.Empty(t, publishing.Routes)
	require.Empty(t, publishing.Jobs)
	require.Empty(t, publishing.Health)
	require.Len(t, publishing.Permissions, 6)
	for _, permission := range publishing.Permissions {
		require.False(t, permission.Active, permission.Key)
	}
}
