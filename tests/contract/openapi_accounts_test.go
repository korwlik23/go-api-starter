package contract_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/stretchr/testify/require"
)

func TestCurrentAccountContract(t *testing.T) {
	loader := openapi3.NewLoader()
	loader.IsExternalRefsAllowed = true
	document, err := loader.LoadFromFile(filepath.Join("..", "..", "openapi", "root.yaml"))
	require.NoError(t, err)
	require.NoError(t, document.Validate(context.Background()))

	pathItem := document.Paths.Find("/accounts/current")
	require.NotNil(t, pathItem)
	operation := pathItem.Get
	require.NotNil(t, operation)
	require.Equal(t, "getCurrentAccount", operation.OperationID)
	require.Equal(t, []interface{}{"admin"}, operation.Extensions["x-surfaces"])
	require.Contains(t, (*operation.Security)[0], "SessionCookie")

	response := operation.Responses.Value("200")
	require.NotNil(t, response)
	schema := response.Value.Content.Get("application/json").Schema.Value
	require.ElementsMatch(t, []string{"id", "name", "slug"}, schema.Required)
	require.NotContains(t, schema.Properties, "database_id")
}
