package application

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestValidateRoleAssignmentRequiresExactSystemGrantPermission(t *testing.T) {
	t.Parallel()

	permissions := map[string]bool{
		PermissionRolePermissionGrant: true,
	}
	err := ValidateRoleAssignment(RoleAssignmentPolicyInput{
		GrantorPermissions: permissions,
		Scope:              AssignmentScopeSystem,
		RoleOwner:          RoleOwnerSystem,
		TargetActive:       true,
		ModuleEnabled:      true,
	})
	require.ErrorIs(t, err, ErrRoleAssignmentDenied)

	permissions["authorization.role_permissions.grant.system"] = true
	permissions["authorization.role_assignments.assign.system"] = true
	err = ValidateRoleAssignment(RoleAssignmentPolicyInput{
		GrantorPermissions: permissions,
		Scope:              AssignmentScopeSystem,
		RoleOwner:          RoleOwnerSystem,
		TargetActive:       true,
		ModuleEnabled:      true,
	})
	require.NoError(t, err)
}

func TestValidateRoleAssignmentFailsClosedAcrossScopesAndDuplicates(t *testing.T) {
	t.Parallel()

	permissions := map[string]bool{
		PermissionRoleAssignmentAssign: true,
	}
	require.ErrorIs(t, ValidateRoleAssignment(RoleAssignmentPolicyInput{
		GrantorPermissions: permissions,
		Scope:              AssignmentScopeSystem,
		RoleOwner:          RoleOwnerAccount,
		TargetActive:       true,
		ModuleEnabled:      true,
	}), ErrRoleAssignmentDenied)
	require.ErrorIs(t, ValidateRoleAssignment(RoleAssignmentPolicyInput{
		GrantorPermissions: permissions,
		Scope:              AssignmentScopeAccount,
		RoleOwner:          RoleOwnerAccount,
		RoleAccountID:      "account-1",
		TargetAccountID:    "account-2",
		TargetActive:       true,
		ModuleEnabled:      true,
	}), ErrRoleAssignmentDenied)
	require.ErrorIs(t, ValidateRoleAssignment(RoleAssignmentPolicyInput{
		GrantorPermissions: permissions,
		Scope:              AssignmentScopeSystem,
		RoleOwner:          RoleOwnerSystem,
		TargetActive:       true,
		ModuleEnabled:      true,
		AlreadyAssigned:    true,
	}), ErrRoleAssignmentDenied)
}
