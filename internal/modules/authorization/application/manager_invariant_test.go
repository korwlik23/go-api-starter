package application

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLastManagerInvariantUsesEffectivePermissionsNotRoleNames(t *testing.T) {
	t.Parallel()

	required := []string{
		"authorization.role_assignments.assign.system",
		"authorization.role_assignments.revoke.system",
	}

	manager := map[string]bool{
		required[0]: true,
		required[1]: true,
	}
	targetAfter := map[string]bool{required[0]: true}
	other := map[string]bool{required[0]: true}

	require.ErrorIs(t, ValidateLastManagerRevoke(targetAfter, []map[string]bool{other}, required), ErrLastManager)
	require.NoError(t, ValidateLastManagerRevoke(manager, []map[string]bool{manager}, required))
}

func TestLastManagerInvariantDoesNotInferPermissionFromOwnOrRoleName(t *testing.T) {
	t.Parallel()

	required := []string{"authorization.role_assignments.revoke.system"}
	target := map[string]bool{
		"authorization.role_assignments.revoke.own": true,
		"role.name.manager":                         true,
	}

	require.ErrorIs(
		t,
		ValidateLastManagerRevoke(target, nil, required),
		ErrLastManager,
	)
}
