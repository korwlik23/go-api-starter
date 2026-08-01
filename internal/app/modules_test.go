package app

import (
	"testing"

	"github.com/korwlik23/go-api-starter/internal/modular"
	"github.com/stretchr/testify/require"
)

func TestCoreModuleRegistryIsCanonicalAndDependencyOrdered(t *testing.T) {
	t.Parallel()

	registry, err := Modules()
	require.NoError(t, err)
	require.Equal(
		t,
		[]string{
			"identity",
			"accounts",
			"audit",
			"authorization",
			"localization",
			"operations",
		},
		coreModuleIDs(registry.Ordered()),
	)

	for _, manifest := range registry.Ordered() {
		require.Equal(t, modular.TierCore, manifest.Tier)
		require.False(t, manifest.Disableable)
		require.Len(t, manifest.Migrations, 3)
	}
}

func TestCoreModuleRegistryOwnsRequiredPermissionCatalog(t *testing.T) {
	t.Parallel()

	registry, err := Modules()
	require.NoError(t, err)

	expected := []string{
		"accounts.current.read.own",
		"accounts.memberships.read.system",
		"accounts.memberships.manage.system",
		"audit.events.read.system",
		"authorization.permissions.read.own",
		"authorization.roles.read.system",
		"authorization.roles.manage.system",
		"authorization.role_permissions.read.system",
		"authorization.role_assignments.assign.system",
		"authorization.role_assignments.read.system",
		"authorization.role_assignments.revoke.system",
		"authorization.role_permissions.grant.system",
		"identity.users.read.system",
		"identity.users.manage.system",
		"localization.locales.manage.system",
		"localization.translations.update.system",
		"operations.foundation.read.system",
	}
	for _, key := range expected {
		permission, exists := registry.Permission(key)
		require.True(t, exists, key)
		require.Equal(t, modular.TierCore, permission.Tier)
	}
}

func TestAuditManifestPinsForeignKeyParents(t *testing.T) {
	t.Parallel()

	registry, err := Modules()
	require.NoError(t, err)
	audit, exists := registry.Manifest("audit")
	require.True(t, exists)
	require.ElementsMatch(t, []string{"accounts", "identity"}, audit.Dependencies)
	require.Contains(t, audit.ProvidedCapabilities, "audit.append")
}

func coreModuleIDs(manifests []modular.Manifest) []string {
	ids := make([]string, 0, len(manifests))
	for _, manifest := range manifests {
		ids = append(ids, manifest.ID)
	}
	return ids
}
