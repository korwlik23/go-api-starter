package gorm

import (
	"testing"

	authorizationapp "github.com/korwlik23/go-api-starter/internal/modules/authorization/application"
	"github.com/stretchr/testify/require"
)

func TestHasAllSystemManagerPermissionsRequiresExactOperationPermissions(t *testing.T) {
	require.True(t, hasAllSystemManagerPermissions([]string{
		authorizationapp.PermissionRoleAssignmentAssign,
		authorizationapp.PermissionRoleAssignmentRevoke,
	}))
	require.False(t, hasAllSystemManagerPermissions([]string{
		authorizationapp.PermissionRoleAssignmentAssign,
	}))
	require.False(t, hasAllSystemManagerPermissions([]string{
		"authorization.role_assignments.manager.system",
		authorizationapp.PermissionRoleAssignmentRevoke,
	}))
}
