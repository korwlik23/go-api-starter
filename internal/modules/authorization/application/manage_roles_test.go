package application

import (
	"context"
	"errors"
	"testing"

	authorizationdomain "github.com/korwlik23/go-api-starter/internal/modules/authorization/domain"
	"github.com/stretchr/testify/require"
)

func TestManageRolesRejectsCreateWithoutExactSystemPermission(t *testing.T) {
	repository := &roleManagementRepositoryFake{}
	service, err := NewRoleManagementService(
		&roleManagementTransactorFake{},
		roleManagementAuthorizerFake{allowed: false},
		repository,
	)
	require.NoError(t, err)

	_, err = service.Create(context.Background(), CreateRoleCommand{
		ActorUserID: "actor-1",
		AccountID:   "account-1",
		Name:        "Editors",
	})
	require.ErrorIs(t, err, ErrRoleManagementDenied)
	require.False(t, repository.created)
}

func TestManageRolesCreatesInsideTransactionAndAuditsBeforeCommit(t *testing.T) {
	repository := &roleManagementRepositoryFake{createdRole: authorizationdomain.Role{
		ID: "role-1", OwnerType: authorizationdomain.RoleOwnerAccount,
		AccountID: "account-1", Name: "Editors", Version: 1,
	}}
	auditor := &roleManagementAuditorFake{}
	service, err := NewRoleManagementService(
		&roleManagementTransactorFake{},
		roleManagementAuthorizerFake{allowed: true},
		repository,
		auditor,
	)
	require.NoError(t, err)

	result, err := service.Create(context.Background(), CreateRoleCommand{
		ActorUserID: "actor-1", AccountID: "account-1", Name: "Editors", RequestID: "request-1",
	})
	require.NoError(t, err)
	require.Equal(t, repository.createdRole, result)
	require.True(t, repository.created)
	require.True(t, auditor.called)
	require.Equal(t, "authorization.role.created", auditor.event.Action)
	require.Equal(t, "request-1", auditor.event.RequestID)
}

func TestManageRolesUpdatesWithExpectedVersion(t *testing.T) {
	repository := &roleManagementRepositoryFake{updatedRole: authorizationdomain.Role{
		ID: "role-1", OwnerType: authorizationdomain.RoleOwnerAccount,
		AccountID: "account-1", Name: "Reviewers", Version: 4,
	}}
	service, err := NewRoleManagementService(
		&roleManagementTransactorFake{},
		roleManagementAuthorizerFake{allowed: true},
		repository,
	)
	require.NoError(t, err)

	result, err := service.Update(context.Background(), UpdateRoleCommand{
		ActorUserID: "actor-1", AccountID: "account-1", RoleID: "role-1",
		Name: "Reviewers", ExpectedVersion: 3,
	})
	require.NoError(t, err)
	require.Equal(t, repository.updatedRole, result)
	require.Equal(t, int64(3), repository.expectedVersion)
}

func TestManageRolesRollsBackWhenAuditFails(t *testing.T) {
	transactor := &roleManagementTransactorFake{}
	repository := &roleManagementRepositoryFake{createdRole: authorizationdomain.Role{
		ID: "role-1", OwnerType: authorizationdomain.RoleOwnerAccount,
		AccountID: "account-1", Name: "Editors", Version: 1,
	}}
	auditor := &roleManagementAuditorFake{err: errors.New("audit unavailable")}
	service, err := NewRoleManagementService(
		transactor,
		roleManagementAuthorizerFake{allowed: true},
		repository,
		auditor,
	)
	require.NoError(t, err)

	_, err = service.Create(context.Background(), CreateRoleCommand{
		ActorUserID: "actor-1", AccountID: "account-1", Name: "Editors",
	})
	require.Error(t, err)
	require.True(t, transactor.rolledBack)
}

type roleManagementTransactorFake struct {
	called     bool
	rolledBack bool
}

func (fake *roleManagementTransactorFake) WithinTransaction(
	ctx context.Context,
	operation func(context.Context) error,
) error {
	fake.called = true
	err := operation(ctx)
	fake.rolledBack = err != nil
	return err
}

type roleManagementAuthorizerFake struct {
	allowed bool
}

func (fake roleManagementAuthorizerFake) Allowed(
	context.Context,
	string,
	authorizationdomain.PermissionKey,
) (bool, error) {
	return fake.allowed, nil
}

type roleManagementRepositoryFake struct {
	created         bool
	createdRole     authorizationdomain.Role
	updatedRole     authorizationdomain.Role
	expectedVersion int64
	updateCalled    bool
	createErr       error
	updateErr       error
}

func (fake *roleManagementRepositoryFake) CreateAccountRole(
	context.Context,
	string,
	string,
) (authorizationdomain.Role, error) {
	fake.created = true
	return fake.createdRole, fake.createErr
}

func (fake *roleManagementRepositoryFake) UpdateAccountRoleName(
	_ context.Context,
	_ string,
	_ string,
	expectedVersion int64,
	_ string,
) (authorizationdomain.Role, error) {
	fake.updateCalled = true
	fake.expectedVersion = expectedVersion
	return fake.updatedRole, fake.updateErr
}

type roleManagementAuditorFake struct {
	called bool
	event  RoleManagementAuditEvent
	err    error
}

func (fake *roleManagementAuditorFake) RecordRoleChange(
	_ context.Context,
	event RoleManagementAuditEvent,
) error {
	fake.called = true
	fake.event = event
	return fake.err
}
