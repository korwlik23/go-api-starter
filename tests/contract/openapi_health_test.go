package contract_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/stretchr/testify/require"
)

func TestLivenessContract(t *testing.T) {
	loader := openapi3.NewLoader()
	loader.IsExternalRefsAllowed = true
	document, err := loader.LoadFromFile(filepath.Join("..", "..", "openapi", "root.yaml"))
	require.NoError(t, err)
	require.NoError(t, document.Validate(context.Background()))

	pathItem := document.Paths.Find("/livez")
	require.NotNil(t, pathItem)
	operation := pathItem.Get
	require.NotNil(t, operation)
	require.Equal(t, "getLiveness", operation.OperationID)
	require.Equal(t, "{scheme}://{host}", (*operation.Servers)[0].URL)
	require.Empty(t, *operation.Security)

	schema := operation.Responses.Value("200").Value.Content.Get("application/json").Schema.Value
	require.ElementsMatch(t, []string{"status"}, schema.Required)
	require.Equal(t, []interface{}{"ok"}, schema.Properties["status"].Value.Enum)
	require.NotContains(t, schema.Properties, "dependencies")
	require.NotContains(t, schema.Properties, "checks")
	require.NotContains(t, schema.Properties, "secrets")
}

func TestReadinessContract(t *testing.T) {
	loader := openapi3.NewLoader()
	loader.IsExternalRefsAllowed = true
	document, err := loader.LoadFromFile(filepath.Join("..", "..", "openapi", "root.yaml"))
	require.NoError(t, err)
	require.NoError(t, document.Validate(context.Background()))

	pathItem := document.Paths.Find("/readyz")
	require.NotNil(t, pathItem)
	operation := pathItem.Get
	require.NotNil(t, operation)
	require.Equal(t, "getReadiness", operation.OperationID)
	require.Equal(t, "{scheme}://{host}", (*operation.Servers)[0].URL)

	ready := operation.Responses.Value("200").Value.Content.Get("application/json").Schema.Value
	unavailable := operation.Responses.Value("503").Value.Content.Get("application/json").Schema.Value
	require.Equal(t, []interface{}{"ready"}, ready.Properties["status"].Value.Enum)
	require.Equal(t, []interface{}{"unavailable"}, unavailable.Properties["status"].Value.Enum)
	for _, schema := range []*openapi3.Schema{ready, unavailable} {
		require.ElementsMatch(t, []string{"status"}, schema.Required)
		require.NotContains(t, schema.Properties, "dependencies")
		require.NotContains(t, schema.Properties, "checks")
		require.NotContains(t, schema.Properties, "secrets")
	}
}
