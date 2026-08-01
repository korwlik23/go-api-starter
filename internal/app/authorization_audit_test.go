package app

import (
	"context"
	"testing"

	auditapp "github.com/korwlik23/go-api-starter/internal/modules/audit/application"
	auditdomain "github.com/korwlik23/go-api-starter/internal/modules/audit/domain"
	authorizationapp "github.com/korwlik23/go-api-starter/internal/modules/authorization/application"
	"github.com/stretchr/testify/require"
)

func TestAuthorizationAuditAdapterMapsRoleAssignmentEvent(t *testing.T) {
	appender := &authorizationAuditAppenderFake{}
	adapter := authorizationAuditAdapter{appender: appender}

	err := adapter.RecordRoleAssignment(context.Background(), authorizationapp.RoleAssignmentAuditEvent{
		Scope: "account", AccountID: "account-1", ActorUserID: "actor-1", RequestID: "request-1",
		Action: "authorization.role.assignment.assigned", AssignmentID: "assignment-1",
		RoleID: "role-1", TargetUserID: "target-1",
	})

	require.NoError(t, err)
	require.Equal(t, auditdomain.ScopeAccount, appender.command.Scope)
	require.Equal(t, "account-1", appender.command.AccountID)
	require.Equal(t, auditdomain.Actor{Type: auditdomain.ActorUser, ID: "actor-1"}, appender.command.Actor)
	require.Equal(t, "request-1", appender.command.RequestID)
	require.Equal(t, "assignment-1", appender.command.Data["assignment_id"])
	require.Equal(t, "role-1", appender.command.Data["role_id"])
	require.Equal(t, "target-1", appender.command.Data["target_user_id"])
}

func TestAuthorizationAuditAdapterMapsRolePermissionGrant(t *testing.T) {
	appender := &authorizationAuditAppenderFake{}
	adapter := authorizationAuditAdapter{appender: appender}

	err := adapter.RecordRolePermissionGrant(context.Background(), authorizationapp.GrantRolePermissionAuditEvent{
		Scope: "system", AccountID: "", ActorUserID: "actor-1", RequestID: "request-2",
		Action: "authorization.role.permission.granted", RoleID: "role-1",
		PermissionKey: "content.items.read.system",
	})

	require.NoError(t, err)
	require.Equal(t, auditdomain.ScopeSystem, appender.command.Scope)
	require.Equal(t, auditdomain.Actor{Type: auditdomain.ActorUser, ID: "actor-1"}, appender.command.Actor)
	require.Equal(t, "content.items.read.system", appender.command.Data["permission_key"])
	require.Equal(t, "role-1", appender.command.Data["role_id"])
}

func TestAuthorizationAuditAdapterMapsRoleChangeEvent(t *testing.T) {
	appender := &authorizationAuditAppenderFake{}
	adapter := authorizationAuditAdapter{appender: appender}

	err := adapter.RecordRoleChange(context.Background(), authorizationapp.RoleManagementAuditEvent{
		Scope: "account", AccountID: "account-1", ActorUserID: "actor-1", RequestID: "request-3",
		Action: "authorization.role.updated", RoleID: "role-1",
	})

	require.NoError(t, err)
	require.Equal(t, auditdomain.ScopeAccount, appender.command.Scope)
	require.Equal(t, "account-1", appender.command.AccountID)
	require.Equal(t, "actor-1", appender.command.Actor.ID)
	require.Equal(t, "request-3", appender.command.RequestID)
	require.Equal(t, "authorization.role.updated", appender.command.Action)
	require.Equal(t, "role-1", appender.command.Data["role_id"])
}

type authorizationAuditAppenderFake struct {
	command auditapp.AppendCommand
}

func (fake *authorizationAuditAppenderFake) Execute(
	_ context.Context,
	command auditapp.AppendCommand,
) (auditdomain.Event, error) {
	fake.command = command
	return auditdomain.Event{}, nil
}
