package application

import (
	"context"
	"testing"

	"github.com/korwlik23/go-api-starter/internal/platform/database"
	"github.com/stretchr/testify/require"
)

func TestGrantRolePermissionExecutesRepositoryInsideTransaction(t *testing.T) {
	t.Parallel()

	transactor := &grantTransactorFake{}
	repository := &grantRepositoryFake{
		result: GrantRolePermissionResult{RoleID: "role-1", Version: 2},
	}
	service, err := NewGrantRolePermissionService(transactor, repository)
	require.NoError(t, err)

	result, err := service.Execute(context.Background(), GrantRolePermissionCommand{
		GrantorUserID:   "user-1",
		RoleID:          "role-1",
		PermissionKey:   "content.items.read.system",
		ExpectedVersion: 1,
	})
	require.NoError(t, err)
	require.Equal(t, repository.result, result)
	require.True(t, transactor.called)
	require.True(t, repository.called)
}

func TestGrantRolePermissionReturnsRepositoryErrorAndRollsBack(t *testing.T) {
	t.Parallel()

	transactor := &grantTransactorFake{}
	repository := &grantRepositoryFake{err: ErrGrantConflict}
	service, err := NewGrantRolePermissionService(transactor, repository)
	require.NoError(t, err)

	_, err = service.Execute(context.Background(), GrantRolePermissionCommand{
		GrantorUserID:   "user-1",
		RoleID:          "role-1",
		PermissionKey:   "content.items.read.system",
		ExpectedVersion: 1,
	})
	require.ErrorIs(t, err, ErrGrantConflict)
	require.True(t, transactor.called)
	require.True(t, transactor.rolledBack)
}

type grantTransactorFake struct {
	called     bool
	rolledBack bool
}

func (fake *grantTransactorFake) WithinTransaction(
	ctx context.Context,
	operation func(context.Context) error,
) error {
	fake.called = true
	err := operation(ctx)
	if err != nil {
		fake.rolledBack = true
	}
	return err
}

var _ database.Transactor = (*grantTransactorFake)(nil)

type grantRepositoryFake struct {
	called bool
	result GrantRolePermissionResult
	err    error
}

func (fake *grantRepositoryFake) GrantRolePermission(
	context.Context,
	GrantRolePermissionCommand,
) (GrantRolePermissionResult, error) {
	fake.called = true
	if fake.err != nil {
		return GrantRolePermissionResult{}, fake.err
	}
	return fake.result, nil
}
