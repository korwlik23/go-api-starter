package application

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGrantRolePermissionPolicyRequiresExactDelegableTarget(t *testing.T) {
	t.Parallel()

	base := GrantRolePermissionInput{
		GrantorPermissions: map[string]bool{
			PermissionRolePermissionGrant: true,
			"content.items.read.system":   true,
		},
		RoleOwner:           RoleOwnerSystem,
		TargetPermissionKey: "content.items.read.system",
		TargetScope:         PermissionScopeSystem,
		TargetActive:        true,
		TargetDelegable:     true,
		ModuleEnabled:       true,
	}

	require.NoError(t, ValidateGrantRolePermission(base))

	tests := []struct {
		name   string
		mutate func(*GrantRolePermissionInput)
	}{
		{
			name:   "inactive target",
			mutate: func(input *GrantRolePermissionInput) { input.TargetActive = false },
		},
		{
			name:   "non delegable target",
			mutate: func(input *GrantRolePermissionInput) { input.TargetDelegable = false },
		},
		{
			name:   "disabled module",
			mutate: func(input *GrantRolePermissionInput) { input.ModuleEnabled = false },
		},
		{
			name: "missing exact target permission",
			mutate: func(input *GrantRolePermissionInput) {
				input.GrantorPermissions = map[string]bool{
					PermissionRolePermissionGrant: true,
					"content.items.read.own":      true,
				}
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			candidate := base
			test.mutate(&candidate)
			require.ErrorIs(t, ValidateGrantRolePermission(candidate), ErrGrantDenied)
		})
	}
}

func TestGrantRolePermissionPolicyMatchesRoleAndPermissionScope(t *testing.T) {
	t.Parallel()

	accountRole := GrantRolePermissionInput{
		GrantorPermissions: map[string]bool{
			PermissionRolePermissionGrant: true,
			"content.items.read.any":      true,
		},
		RoleOwner:           RoleOwnerAccount,
		TargetPermissionKey: "content.items.read.any",
		TargetScope:         PermissionScopeAny,
		TargetActive:        true,
		TargetDelegable:     true,
		ModuleEnabled:       true,
	}
	require.NoError(t, ValidateGrantRolePermission(accountRole))

	accountRole.TargetScope = PermissionScopeSystem
	require.ErrorIs(t, ValidateGrantRolePermission(accountRole), ErrGrantDenied)
}
