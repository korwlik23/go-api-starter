package contract_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/stretchr/testify/require"
)

func TestEffectivePermissionContract(t *testing.T) {
	loader := openapi3.NewLoader()
	loader.IsExternalRefsAllowed = true
	document, err := loader.LoadFromFile(filepath.Join("..", "..", "openapi", "root.yaml"))
	require.NoError(t, err)
	require.NoError(t, document.Validate(context.Background()))

	pathItem := document.Paths.Find("/authorization/effective-permissions")
	require.NotNil(t, pathItem)
	operation := pathItem.Get
	require.NotNil(t, operation)
	require.Equal(t, "getEffectivePermissions", operation.OperationID)
	require.Equal(t, []interface{}{"admin"}, operation.Extensions["x-surfaces"])

	response := operation.Responses.Value("200")
	require.NotNil(t, response)
	schema := response.Value.Content.Get("application/json").Schema.Value
	require.ElementsMatch(t, []string{"account_id", "permissions"}, schema.Required)
	permissions := schema.Properties["permissions"].Value
	require.True(t, permissions.UniqueItems)
	require.True(t, permissions.Items.Value.Type.Is("string"))
}
