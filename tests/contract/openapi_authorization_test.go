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

func TestPermissionGrantContract(t *testing.T) {
	loader := openapi3.NewLoader()
	loader.IsExternalRefsAllowed = true
	document, err := loader.LoadFromFile(filepath.Join("..", "..", "openapi", "root.yaml"))
	require.NoError(t, err)
	require.NoError(t, document.Validate(context.Background()))

	pathItem := document.Paths.Find("/authorization/roles/{role_id}/permissions")
	require.NotNil(t, pathItem)
	operation := pathItem.Post
	require.NotNil(t, operation)
	require.Equal(t, "grantRolePermission", operation.OperationID)
	require.Equal(t, []interface{}{"admin"}, operation.Extensions["x-surfaces"])
	require.Contains(t, (*operation.Security)[0], "CSRFToken")

	request := operation.RequestBody.Value.Content.Get("application/json").Schema.Value
	require.ElementsMatch(t, []string{"permission_key", "expected_version"}, request.Required)
	require.True(t, request.Properties["expected_version"].Value.Type.Is("integer"))

	response := operation.Responses.Value("200")
	require.NotNil(t, response)
	schema := response.Value.Content.Get("application/json").Schema.Value
	require.Subset(t, schema.Required, []string{"role_id", "permission_key", "version"})
	require.NotNil(t, operation.Responses.Value("409"))
}

func TestRoleAssignmentContract(t *testing.T) {
	loader := openapi3.NewLoader()
	loader.IsExternalRefsAllowed = true
	document, err := loader.LoadFromFile(filepath.Join("..", "..", "openapi", "root.yaml"))
	require.NoError(t, err)
	require.NoError(t, document.Validate(context.Background()))

	pathItem := document.Paths.Find("/authorization/roles/{role_id}/assignments")
	require.NotNil(t, pathItem)
	operation := pathItem.Post
	require.NotNil(t, operation)
	require.Equal(t, "assignRole", operation.OperationID)

	request := operation.RequestBody.Value.Content.Get("application/json").Schema.Value
	require.Len(t, request.OneOf, 2)
	require.NotNil(t, request.Discriminator)
	require.Equal(t, "scope", request.Discriminator.PropertyName)
	require.Contains(t, request.Discriminator.Mapping, "account")
	require.Contains(t, request.Discriminator.Mapping, "system")

	response := operation.Responses.Value("200")
	require.NotNil(t, response)
	schema := response.Value.Content.Get("application/json").Schema.Value
	require.Subset(t, schema.Required, []string{"assignment_id", "role_id", "version"})
}

func TestRoleRevokeContract(t *testing.T) {
	loader := openapi3.NewLoader()
	loader.IsExternalRefsAllowed = true
	document, err := loader.LoadFromFile(filepath.Join("..", "..", "openapi", "root.yaml"))
	require.NoError(t, err)
	require.NoError(t, document.Validate(context.Background()))

	pathItem := document.Paths.Find("/authorization/assignments/{assignment_id}")
	require.NotNil(t, pathItem)
	operation := pathItem.Delete
	require.NotNil(t, operation)
	require.Equal(t, "revokeRoleAssignment", operation.OperationID)
	require.Contains(t, (*operation.Security)[0], "CSRFToken")

	request := operation.RequestBody.Value.Content.Get("application/json").Schema.Value
	require.ElementsMatch(t, []string{"expected_version"}, request.Required)

	response := operation.Responses.Value("200")
	require.NotNil(t, response)
	schema := response.Value.Content.Get("application/json").Schema.Value
	require.Subset(t, schema.Required, []string{"assignment_id", "revoked", "version"})
	require.NotNil(t, operation.Responses.Value("409"), "last-manager and version conflicts must use the stable conflict response")
}

func TestRoleAdministrationContract(t *testing.T) {
	loader := openapi3.NewLoader()
	loader.IsExternalRefsAllowed = true
	document, err := loader.LoadFromFile(filepath.Join("..", "..", "openapi", "root.yaml"))
	require.NoError(t, err)
	require.NoError(t, document.Validate(context.Background()))

	roles := document.Paths.Find("/authorization/roles")
	require.NotNil(t, roles)
	require.NotNil(t, roles.Get)
	require.NotNil(t, roles.Post)
	require.Equal(t, "listAuthorizationRoles", roles.Get.OperationID)
	require.Equal(t, "createAuthorizationRole", roles.Post.OperationID)
	require.Equal(t, []interface{}{"admin"}, roles.Get.Extensions["x-surfaces"])
	require.Equal(t, []interface{}{"admin"}, roles.Post.Extensions["x-surfaces"])

	role := document.Paths.Find("/authorization/roles/{role_id}")
	require.NotNil(t, role)
	require.NotNil(t, role.Patch)
	require.Equal(t, "updateAuthorizationRole", role.Patch.OperationID)

	permissions := document.Paths.Find("/authorization/roles/{role_id}/permissions")
	require.NotNil(t, permissions)
	require.NotNil(t, permissions.Get)
	require.Equal(t, "listRolePermissions", permissions.Get.OperationID)

	assignments := document.Paths.Find("/authorization/roles/{role_id}/assignments")
	require.NotNil(t, assignments)
	require.NotNil(t, assignments.Get)
	require.Equal(t, "listRoleAssignments", assignments.Get.OperationID)
}

func TestPrivilegeChangeSessionOutcomeContract(t *testing.T) {
	loader := openapi3.NewLoader()
	loader.IsExternalRefsAllowed = true
	document, err := loader.LoadFromFile(filepath.Join("..", "..", "openapi", "root.yaml"))
	require.NoError(t, err)
	require.NoError(t, document.Validate(context.Background()))

	operations := []*openapi3.Operation{
		document.Paths.Find("/authorization/roles/{role_id}/permissions").Post,
		document.Paths.Find("/authorization/roles/{role_id}/assignments").Post,
		document.Paths.Find("/authorization/assignments/{assignment_id}").Delete,
	}
	for _, operation := range operations {
		schema := operation.Responses.Value("200").Value.Content.Get("application/json").Schema.Value
		require.Contains(t, schema.Required, "affected_principals")
		require.Contains(t, schema.Required, "session_outcome")
		outcome := schema.Properties["session_outcome"].Value
		require.ElementsMatch(t, []string{"current_principal", "other_principals"}, outcome.Required)
		require.Equal(t, []interface{}{"unchanged", "rotated"}, outcome.Properties["current_principal"].Value.Enum)
		require.Equal(t, []interface{}{"unchanged", "invalidated"}, outcome.Properties["other_principals"].Value.Enum)
	}
}
