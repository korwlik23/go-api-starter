package application

import (
	"context"
	"testing"
	"time"

	authdomain "github.com/korwlik23/go-api-starter/internal/modules/authorization/domain"
	identitydomain "github.com/korwlik23/go-api-starter/internal/modules/identity/domain"
	"github.com/korwlik23/go-api-starter/internal/modules/identity/ports"
	"github.com/stretchr/testify/require"
)

func TestAdminUserStatusRequiresManagePermissionAndAuditsInsideTransaction(t *testing.T) {
	commands := &adminUserCommandStub{user: identitydomain.User{ID: "user-1", Status: identitydomain.UserStatusDisabled, AuthorizationVersion: 2, CreatedAt: time.Now(), UpdatedAt: time.Now()}}
	transactor := &adminUserTransactorStub{}
	auditor := &adminUserAuditorStub{}
	service, err := NewAdminUserService(transactor, adminUserAuthorizerStub{allowed: true}, adminUserQueryStub{}, commands, auditor)
	require.NoError(t, err)
	result, err := service.UpdateStatus(context.Background(), UpdateAdminUserStatusCommand{ActorUserID: "actor", UserID: "user-1", ExpectedVersion: 1, Status: identitydomain.UserStatusDisabled, RequestID: "req"})
	require.NoError(t, err)
	require.Equal(t, commands.user, result)
	require.True(t, transactor.called)
	require.Equal(t, "user-1", auditor.event.UserID)
}

func TestAdminUserStatusRejectsDeniedActor(t *testing.T) {
	commands := &adminUserCommandStub{}
	service, err := NewAdminUserService(&adminUserTransactorStub{}, adminUserAuthorizerStub{allowed: false}, adminUserQueryStub{}, commands)
	require.NoError(t, err)
	_, err = service.UpdateStatus(context.Background(), UpdateAdminUserStatusCommand{ActorUserID: "actor", UserID: "user-1", ExpectedVersion: 1, Status: identitydomain.UserStatusActive})
	require.ErrorIs(t, err, ErrAdminUserDenied)
	require.False(t, commands.called)
}

type adminUserAuthorizerStub struct{ allowed bool }

func (stub adminUserAuthorizerStub) Allowed(context.Context, string, authdomain.PermissionKey) (bool, error) {
	return stub.allowed, nil
}

type adminUserTransactorStub struct{ called bool }

func (stub *adminUserTransactorStub) WithinTransaction(ctx context.Context, fn func(context.Context) error) error {
	stub.called = true
	return fn(ctx)
}

type adminUserQueryStub struct{}

func (adminUserQueryStub) ListAdminUsers(context.Context, ports.AdminUserListFilter) (ports.AdminUserPage, error) {
	return ports.AdminUserPage{}, nil
}
func (adminUserQueryStub) FindAdminUser(context.Context, string) (ports.AdminUserDetail, error) {
	return ports.AdminUserDetail{}, nil
}

type adminUserCommandStub struct {
	called bool
	user   identitydomain.User
}

func (stub *adminUserCommandStub) UpdateUserStatus(context.Context, string, int64, identitydomain.UserStatus, time.Time) (identitydomain.User, error) {
	stub.called = true
	return stub.user, nil
}

type adminUserAuditorStub struct{ event AdminUserAuditEvent }

func (stub *adminUserAuditorStub) RecordUserStatusChange(_ context.Context, event AdminUserAuditEvent) error {
	stub.event = event
	return nil
}
