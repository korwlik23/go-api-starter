package publishing

import (
	"testing"

	"github.com/korwlik23/go-api-starter/internal/modular"
	"github.com/stretchr/testify/require"
)

func TestM3A1ManifestValidatesAndExposesPublishingSurface(t *testing.T) {
	t.Parallel()

	manifest := Manifest()
	require.NoError(t, modular.ValidateManifest(manifest))
	require.Equal(t, "publishing", manifest.ID)
	require.Equal(t, modular.TierDefault, manifest.Tier)
	require.True(t, manifest.Disableable)
	require.ElementsMatch(t, []string{
		"accounts",
		"authorization",
		"localization",
		"audit",
	}, manifest.Dependencies)
	require.ElementsMatch(t, []string{
		"accounts.resolve",
		"authorization.evaluate",
		"audit.append",
		"database.sql",
		"localization.catalog",
	}, manifest.RequiredCapabilities)
	require.Equal(t, []string{"publishing.content"}, manifest.ProvidedCapabilities)
	require.Len(t, manifest.Migrations, 3)
	require.Contains(t, manifest.Routes, modular.Route{
		Method:     "GET",
		Path:       "/api/v1/public/content/:locale/*path",
		Permission: "",
	})
	require.Contains(t, manifest.Routes, modular.Route{
		Method:     "POST",
		Path:       "/api/v1/public/previews/exchange",
		Permission: "",
	})
	require.Contains(t, manifest.Jobs, modular.Job{ID: "publishing.schedule.process"})
	require.Contains(t, manifest.Health, modular.HealthContributor{
		ID:       "publishing.content",
		Critical: true,
	})
}
