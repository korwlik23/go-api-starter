package publishing

import (
	"testing"

	"github.com/korwlik23/go-api-starter/internal/modular"
	"github.com/stretchr/testify/require"
)

func TestM3A1PermissionsUseStableKeysAndScopes(t *testing.T) {
	t.Parallel()

	permissions := Permissions()
	require.Len(t, permissions, 6)

	expected := map[string]modular.PermissionScope{
		PermissionPagesReadOwn:         modular.ScopeOwn,
		PermissionPagesWriteOwn:        modular.ScopeOwn,
		PermissionPagesReviewAny:       modular.ScopeAny,
		PermissionPagesPublishAny:      modular.ScopeAny,
		PermissionRevisionsReadOwn:     modular.ScopeOwn,
		PermissionRevisionsRollbackOwn: modular.ScopeOwn,
	}
	for _, permission := range permissions {
		scope, exists := expected[permission.Key]
		require.True(t, exists, permission.Key)
		require.Equal(t, scope, permission.Scope, permission.Key)
		require.Equal(t, modular.TierDefault, permission.Tier, permission.Key)
		require.False(t, permission.Delegable, permission.Key)
		delete(expected, permission.Key)
	}
	require.Empty(t, expected)
}
