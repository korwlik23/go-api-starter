package application

import (
	"context"
	"testing"

	"github.com/korwlik23/go-api-starter/internal/platform/database"
	"github.com/stretchr/testify/require"
)

func TestAssignRoleExecutesRepositoryInsideTransaction(t *testing.T) {
	t.Parallel()

	transactions := &assignmentTransactorFake{}
	repository := &roleAssignmentRepositoryFake{
		assignResult: RoleAssignmentResult{
			AssignmentID: "assignment-1",
			RoleID:       "role-1",
			Version:      2,
		},
	}
	service, err := NewRoleAssignmentService(transactions, repository)
	require.NoError(t, err)

	result, err := service.Assign(context.Background(), AssignRoleCommand{
		GrantorUserID:   "user-1",
		RoleID:          "role-1",
		Scope:           AssignmentScopeSystem,
		TargetUserID:    "user-2",
		ExpectedVersion: 1,
	})
	require.NoError(t, err)
	require.Equal(t, repository.assignResult, result)
	require.True(t, transactions.called)
	require.True(t, repository.assignCalled)
}

func TestRevokeRoleReturnsConflictAndRollsBack(t *testing.T) {
	t.Parallel()

	transactions := &assignmentTransactorFake{}
	repository := &roleAssignmentRepositoryFake{revokeErr: ErrRoleAssignmentConflict}
	service, err := NewRoleAssignmentService(transactions, repository)
	require.NoError(t, err)

	_, err = service.Revoke(context.Background(), RevokeRoleCommand{
		GrantorUserID:   "user-1",
		AssignmentID:    "assignment-1",
		ExpectedVersion: 1,
	})
	require.ErrorIs(t, err, ErrRoleAssignmentConflict)
	require.True(t, transactions.rolledBack)
	require.True(t, repository.revokeCalled)
}

func TestRoleAssignmentServiceRejectsInvalidCommands(t *testing.T) {
	t.Parallel()

	transactions := &assignmentTransactorFake{}
	repository := &roleAssignmentRepositoryFake{}
	service, err := NewRoleAssignmentService(transactions, repository)
	require.NoError(t, err)

	_, err = service.Assign(context.Background(), AssignRoleCommand{
		GrantorUserID: "user-1", RoleID: "role-1", Scope: "unknown",
		TargetUserID: "user-2", ExpectedVersion: 1,
	})
	require.ErrorIs(t, err, ErrInvalidRoleAssignmentCommand)
	require.False(t, repository.assignCalled)
}

func TestRoleAssignmentServiceRecordsAuditBeforeTransactionCommits(t *testing.T) {
	t.Parallel()

	transactions := &assignmentTransactorFake{}
	repository := &roleAssignmentRepositoryFake{
		assignResult: RoleAssignmentResult{
			AssignmentID: "assignment-1", RoleID: "role-1", Scope: AssignmentScopeSystem,
			AffectedUserID: "user-2", Version: 2,
		},
	}
	auditor := &roleAssignmentAuditorFake{}
	service, err := NewRoleAssignmentService(transactions, repository, auditor)
	require.NoError(t, err)

	_, err = service.Assign(context.Background(), AssignRoleCommand{
		GrantorUserID: "user-1", RoleID: "role-1", Scope: AssignmentScopeSystem,
		TargetUserID: "user-2", RequestID: "request-1", ExpectedVersion: 1,
	})
	require.NoError(t, err)
	require.True(t, auditor.called)
	require.Equal(t, "authorization.role.assignment.assigned", auditor.event.Action)
	require.Equal(t, "request-1", auditor.event.RequestID)
	require.Equal(t, "user-1", auditor.event.ActorUserID)
}

func TestRoleAssignmentServiceRollsBackWhenAuditFails(t *testing.T) {
	t.Parallel()

	transactions := &assignmentTransactorFake{}
	repository := &roleAssignmentRepositoryFake{}
	auditor := &roleAssignmentAuditorFake{err: ErrRoleAssignmentConflict}
	service, err := NewRoleAssignmentService(transactions, repository, auditor)
	require.NoError(t, err)

	_, err = service.Revoke(context.Background(), RevokeRoleCommand{
		GrantorUserID: "user-1", AssignmentID: "assignment-1",
		RequestID: "request-2", ExpectedVersion: 1,
	})
	require.ErrorIs(t, err, ErrRoleAssignmentConflict)
	require.True(t, transactions.rolledBack)
	require.True(t, auditor.called)
}

type assignmentTransactorFake struct {
	called     bool
	rolledBack bool
}

func (fake *assignmentTransactorFake) WithinTransaction(
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

var _ database.Transactor = (*assignmentTransactorFake)(nil)

type roleAssignmentRepositoryFake struct {
	assignCalled bool
	revokeCalled bool
	assignResult RoleAssignmentResult
	assignErr    error
	revokeResult RoleRevokeResult
	revokeErr    error
}

type roleAssignmentAuditorFake struct {
	called bool
	event  RoleAssignmentAuditEvent
	err    error
}

func (fake *roleAssignmentAuditorFake) RecordRoleAssignment(
	_ context.Context,
	event RoleAssignmentAuditEvent,
) error {
	fake.called = true
	fake.event = event
	return fake.err
}

func (fake *roleAssignmentRepositoryFake) AssignRole(
	context.Context,
	AssignRoleCommand,
) (RoleAssignmentResult, error) {
	fake.assignCalled = true
	if fake.assignErr != nil {
		return RoleAssignmentResult{}, fake.assignErr
	}
	return fake.assignResult, nil
}

func (fake *roleAssignmentRepositoryFake) RevokeRole(
	context.Context,
	RevokeRoleCommand,
) (RoleRevokeResult, error) {
	fake.revokeCalled = true
	if fake.revokeErr != nil {
		return RoleRevokeResult{}, fake.revokeErr
	}
	return fake.revokeResult, nil
}
