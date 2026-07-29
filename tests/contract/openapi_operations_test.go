package contract_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/stretchr/testify/require"
)

func TestEnabledModuleContract(t *testing.T) {
	loader := openapi3.NewLoader()
	loader.IsExternalRefsAllowed = true
	document, err := loader.LoadFromFile(filepath.Join("..", "..", "openapi", "root.yaml"))
	require.NoError(t, err)
	require.NoError(t, document.Validate(context.Background()))

	pathItem := document.Paths.Find("/operations/modules")
	require.NotNil(t, pathItem)
	operation := pathItem.Get
	require.NotNil(t, operation)
	require.Equal(t, "listEnabledModules", operation.OperationID)
	require.Equal(t, []interface{}{"admin"}, operation.Extensions["x-surfaces"])

	response := operation.Responses.Value("200")
	require.NotNil(t, response)
	item := response.Value.Content.Get("application/json").Schema.Value.Properties["items"].Value.Items.Value
	require.ElementsMatch(t, []string{"id", "enabled", "reconcile_revision"}, item.Required)
	require.NotContains(t, item.Properties, "config")
	require.NotContains(t, item.Properties, "checksum")
}
