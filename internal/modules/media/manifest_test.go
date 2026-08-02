package media

import (
	"testing"

	"github.com/korwlik23/go-api-starter/internal/modular"
	"github.com/stretchr/testify/require"
)

func TestM3B2MediaManifestIsPrivateByDefaultAndPermissionScoped(t *testing.T) {
	t.Parallel()

	manifest := Manifest()
	require.NoError(t, modular.ValidateManifest(manifest))
	require.Equal(t, "media", manifest.ID)
	require.Equal(t, modular.TierDefault, manifest.Tier)
	require.True(t, manifest.Disableable)
	require.ElementsMatch(t, []string{"accounts", "authorization", "audit"}, manifest.Dependencies)
	require.Contains(t, manifest.ProvidedCapabilities, "media.assets")
	require.Len(t, manifest.Permissions, 3)
	require.Contains(t, manifest.Routes, modular.Route{Method: "POST", Path: "/api/v1/media/uploads", Permission: PermissionUploadsWriteOwn})
	require.Contains(t, manifest.Routes, modular.Route{Method: "GET", Path: "/api/v1/media/assets", Permission: PermissionAssetsReadOwn})
}
