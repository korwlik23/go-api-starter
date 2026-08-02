package app

import (
	"testing"

	"github.com/korwlik23/go-api-starter/internal/modular"
	"github.com/stretchr/testify/require"
)

func TestM3B2MediaModuleIsRegisteredWithPrivateAssetContract(t *testing.T) {
	t.Parallel()

	registry, err := Modules()
	require.NoError(t, err)
	manifest, exists := registry.Manifest("media")
	require.True(t, exists)
	require.Equal(t, modular.TierDefault, manifest.Tier)
	require.True(t, manifest.Disableable)
	require.Contains(t, manifest.ProvidedCapabilities, "media.assets")
	for _, key := range []string{"media.assets.read.own", "media.assets.write.own", "media.uploads.write.own"} {
		_, permissionExists := registry.Permission(key)
		require.True(t, permissionExists, key)
	}
}
