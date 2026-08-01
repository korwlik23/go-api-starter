package app

import (
	"context"
	"errors"

	auditapp "github.com/korwlik23/go-api-starter/internal/modules/audit/application"
	auditdomain "github.com/korwlik23/go-api-starter/internal/modules/audit/domain"
	authorizationapp "github.com/korwlik23/go-api-starter/internal/modules/authorization/application"
)

var ErrInvalidAuthorizationAuditAdapter = errors.New("invalid authorization audit adapter")

type authorizationAuditAppender interface {
	Execute(context.Context, auditapp.AppendCommand) (auditdomain.Event, error)
}

type authorizationAuditAdapter struct {
	appender authorizationAuditAppender
}

func (adapter authorizationAuditAdapter) RecordRoleAssignment(
	ctx context.Context,
	event authorizationapp.RoleAssignmentAuditEvent,
) error {
	if adapter.appender == nil || ctx == nil {
		return ErrInvalidAuthorizationAuditAdapter
	}
	scope := auditdomain.Scope(event.Scope)
	switch scope {
	case auditdomain.ScopeSystem:
		if event.AccountID != "" {
			return ErrInvalidAuthorizationAuditAdapter
		}
	case auditdomain.ScopeAccount:
		if event.AccountID == "" {
			return ErrInvalidAuthorizationAuditAdapter
		}
	default:
		return ErrInvalidAuthorizationAuditAdapter
	}
	if event.ActorUserID == "" || event.Action == "" || event.AssignmentID == "" || event.RoleID == "" {
		return ErrInvalidAuthorizationAuditAdapter
	}
	_, err := adapter.appender.Execute(ctx, auditapp.AppendCommand{
		Scope:     scope,
		AccountID: event.AccountID,
		Actor:     auditdomain.Actor{Type: auditdomain.ActorUser, ID: event.ActorUserID},
		Action:    event.Action,
		RequestID: event.RequestID,
		Data: map[string]any{
			"assignment_id":  event.AssignmentID,
			"role_id":        event.RoleID,
			"target_user_id": event.TargetUserID,
			"scope":          event.Scope,
		},
	})
	return err
}

func (adapter authorizationAuditAdapter) RecordRolePermissionGrant(
	ctx context.Context,
	event authorizationapp.GrantRolePermissionAuditEvent,
) error {
	if adapter.appender == nil || ctx == nil {
		return ErrInvalidAuthorizationAuditAdapter
	}
	scope := auditdomain.Scope(event.Scope)
	switch scope {
	case auditdomain.ScopeSystem:
		if event.AccountID != "" {
			return ErrInvalidAuthorizationAuditAdapter
		}
	case auditdomain.ScopeAccount:
		if event.AccountID == "" {
			return ErrInvalidAuthorizationAuditAdapter
		}
	default:
		return ErrInvalidAuthorizationAuditAdapter
	}
	if event.ActorUserID == "" || event.Action == "" || event.RoleID == "" || event.PermissionKey == "" {
		return ErrInvalidAuthorizationAuditAdapter
	}
	_, err := adapter.appender.Execute(ctx, auditapp.AppendCommand{
		Scope: scope, AccountID: event.AccountID,
		Actor:  auditdomain.Actor{Type: auditdomain.ActorUser, ID: event.ActorUserID},
		Action: event.Action, RequestID: event.RequestID,
		Data: map[string]any{
			"permission_key": event.PermissionKey,
			"role_id":        event.RoleID,
			"scope":          event.Scope,
		},
	})
	return err
}

var _ authorizationapp.RoleAssignmentAuditor = authorizationAuditAdapter{}
var _ authorizationapp.GrantRolePermissionAuditor = authorizationAuditAdapter{}
