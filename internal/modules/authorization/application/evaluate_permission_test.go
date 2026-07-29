package application

import (
	"context"
	"testing"

	authorizationdomain "github.com/korwlik23/go-api-starter/internal/modules/authorization/domain"
	"github.com/korwlik23/go-api-starter/internal/modules/authorization/ports"
	"github.com/stretchr/testify/require"
)

func TestEvaluatePermissionDefaultsToDenyAndNeverUsesRoleName(t *testing.T) {
	evaluator := permissionEvaluatorForTest(t, nil, ownershipStub{})
	key := mustPermissionKey(t, "content.articles.read.any")

	allowed, err := evaluator.Execute(context.Background(), PermissionRequest{
		UserID: "user-id", AccountID: "account-id", Permission: key,
	})

	require.NoError(t, err)
	require.False(t, allowed)
}

func TestEvaluatePermissionEnforcesAnyAndOwnWithinResolvedAccount(t *testing.T) {
	anyKey := mustPermissionKey(t, "content.articles.read.any")
	ownKey := mustPermissionKey(t, "content.articles.update.own")
	grants := []ports.EffectiveGrant{
		{
			RoleID: "role-uuid", AccountID: "account-id",
			PermissionID: "permission-any", Key: anyKey,
			Source: ports.GrantSourceAccountRole,
		},
		{
			RoleID: "role-uuid", AccountID: "account-id",
			PermissionID: "permission-own", Key: ownKey,
			Source: ports.GrantSourceAccountRole,
		},
	}
	evaluator := permissionEvaluatorForTest(t, grants, ownershipStub{
		ownerID: "user-id",
		found:   true,
	})

	allowed, err := evaluator.Execute(context.Background(), PermissionRequest{
		UserID: "user-id", AccountID: "account-id", Permission: anyKey,
	})
	require.NoError(t, err)
	require.True(t, allowed)

	allowed, err = evaluator.Execute(context.Background(), PermissionRequest{
		UserID: "user-id", AccountID: "account-id", Permission: ownKey,
		ResourceID: "article-id",
	})
	require.NoError(t, err)
	require.True(t, allowed)

	allowed, err = evaluator.Execute(context.Background(), PermissionRequest{
		UserID: "user-id", AccountID: "other-account", Permission: anyKey,
	})
	require.NoError(t, err)
	require.False(t, allowed)
}

func TestEvaluatePermissionSeparatesSystemAndAccountRoleScopes(t *testing.T) {
	systemKey := mustPermissionKey(t, "system.users.manage.system")
	evaluator := permissionEvaluatorForTest(t, []ports.EffectiveGrant{
		{
			RoleID: "account-role", AccountID: "account-id",
			PermissionID: "system-permission", Key: systemKey,
			Source: ports.GrantSourceAccountRole,
		},
	}, ownershipStub{})

	allowed, err := evaluator.Execute(context.Background(), PermissionRequest{
		UserID: "user-id", AccountID: "account-id", Permission: systemKey,
	})

	require.NoError(t, err)
	require.False(t, allowed)
}

func permissionEvaluatorForTest(
	t *testing.T,
	grants []ports.EffectiveGrant,
	ownership ownershipStub,
) *EvaluatePermission {
	t.Helper()
	evaluator, err := NewEvaluatePermission(
		permissionRepositoryStub{grants: grants},
		ownership,
	)
	require.NoError(t, err)
	return evaluator
}

func mustPermissionKey(
	t *testing.T,
	value string,
) authorizationdomain.PermissionKey {
	t.Helper()
	key, err := authorizationdomain.ParsePermissionKey(value)
	require.NoError(t, err)
	return key
}

type permissionRepositoryStub struct {
	grants []ports.EffectiveGrant
	err    error
}

func (stub permissionRepositoryStub) EffectiveGrants(
	context.Context,
	string,
	string,
) ([]ports.EffectiveGrant, error) {
	return stub.grants, stub.err
}

type ownershipStub struct {
	ownerID string
	found   bool
	err     error
}

func (stub ownershipStub) OwnerWithinAccount(
	context.Context,
	string,
	string,
) (string, bool, error) {
	return stub.ownerID, stub.found, stub.err
}
