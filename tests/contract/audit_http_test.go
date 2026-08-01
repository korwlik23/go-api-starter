package contract_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/stretchr/testify/require"
)

func TestSystemAuditListContract(t *testing.T) {
	loader := openapi3.NewLoader()
	loader.IsExternalRefsAllowed = true
	document, err := loader.LoadFromFile(filepath.Join("..", "..", "openapi", "root.yaml"))
	require.NoError(t, err)
	require.NoError(t, document.Validate(context.Background()))

	operation := document.Paths.Find("/audit/events").Get
	require.NotNil(t, operation)
	require.Equal(t, "listAuditEvents", operation.OperationID)
	require.Equal(t, "audit.events.read.system", operation.Extensions["x-required-permission"])
	require.NotContains(t, operation.Extensions, "x-required-role")
	require.Contains(t, (*operation.Security)[0], "SessionCookie")
	require.NotNil(t, operation.Parameters.GetByInAndName("query", "cursor"))
	require.NotNil(t, operation.Parameters.GetByInAndName("query", "action"))
	require.NotNil(t, operation.Parameters.GetByInAndName("query", "actor_id"))
	require.NotNil(t, operation.Responses.Value("200"))
}
