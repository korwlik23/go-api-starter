package app

import (
	"context"
	"errors"

	accountsapp "github.com/korwlik23/go-api-starter/internal/modules/accounts/application"
	auditapp "github.com/korwlik23/go-api-starter/internal/modules/audit/application"
	auditdomain "github.com/korwlik23/go-api-starter/internal/modules/audit/domain"
	authorizationapp "github.com/korwlik23/go-api-starter/internal/modules/authorization/application"
	identityapp "github.com/korwlik23/go-api-starter/internal/modules/identity/application"
	localizationapp "github.com/korwlik23/go-api-starter/internal/modules/localization/application"
	catalogapp "github.com/korwlik23/go-api-starter/internal/modules/localization/application/catalog"
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

func (adapter authorizationAuditAdapter) RecordRoleChange(
	ctx context.Context,
	event authorizationapp.RoleManagementAuditEvent,
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
	if event.ActorUserID == "" || event.Action == "" || event.RoleID == "" {
		return ErrInvalidAuthorizationAuditAdapter
	}
	_, err := adapter.appender.Execute(ctx, auditapp.AppendCommand{
		Scope: scope, AccountID: event.AccountID,
		Actor:  auditdomain.Actor{Type: auditdomain.ActorUser, ID: event.ActorUserID},
		Action: event.Action, RequestID: event.RequestID,
		Data: map[string]any{
			"role_id": event.RoleID,
			"scope":   event.Scope,
		},
	})
	return err
}

func (adapter authorizationAuditAdapter) RecordUserStatusChange(
	ctx context.Context,
	event identityapp.AdminUserAuditEvent,
) error {
	if adapter.appender == nil || ctx == nil || event.ActorUserID == "" || event.UserID == "" {
		return ErrInvalidAuthorizationAuditAdapter
	}
	_, err := adapter.appender.Execute(ctx, auditapp.AppendCommand{
		Scope:  auditdomain.ScopeSystem,
		Actor:  auditdomain.Actor{Type: auditdomain.ActorUser, ID: event.ActorUserID},
		Action: "identity.user.status.updated", RequestID: event.RequestID,
		Data: map[string]any{"user_id": event.UserID, "status": string(event.Status)},
	})
	return err
}

func (adapter authorizationAuditAdapter) RecordMembershipStatusChange(
	ctx context.Context,
	event accountsapp.MembershipAuditEvent,
) error {
	if adapter.appender == nil || ctx == nil || event.ActorUserID == "" || event.AccountID == "" || event.MembershipID == "" {
		return ErrInvalidAuthorizationAuditAdapter
	}
	_, err := adapter.appender.Execute(ctx, auditapp.AppendCommand{
		Scope: auditdomain.ScopeAccount, AccountID: event.AccountID,
		Actor:  auditdomain.Actor{Type: auditdomain.ActorUser, ID: event.ActorUserID},
		Action: "accounts.membership.status.updated", RequestID: event.RequestID,
		Data: map[string]any{"membership_id": event.MembershipID, "status": string(event.Status)},
	})
	return err
}

func (adapter authorizationAuditAdapter) RecordLocaleChange(
	ctx context.Context,
	event localizationapp.LocaleManagementAuditEvent,
) error {
	if adapter.appender == nil || ctx == nil || event.ActorUserID == "" ||
		event.Action == "" || event.LocaleID == "" || event.Version < 1 {
		return ErrInvalidAuthorizationAuditAdapter
	}
	data := map[string]any{
		"locale_id": event.LocaleID,
		"version":   event.Version,
	}
	if event.PreviousDefaultLocale != "" {
		data["previous_default_locale_id"] = event.PreviousDefaultLocale
	}
	_, err := adapter.appender.Execute(ctx, auditapp.AppendCommand{
		Scope:  auditdomain.ScopeSystem,
		Actor:  auditdomain.Actor{Type: auditdomain.ActorUser, ID: event.ActorUserID},
		Action: event.Action, RequestID: event.RequestID, Data: data,
	})
	return err
}

func (adapter authorizationAuditAdapter) RecordCatalogUpdate(
	ctx context.Context,
	event catalogapp.CatalogAuditEvent,
) error {
	if adapter.appender == nil || ctx == nil || event.ActorUserID == "" ||
		event.Locale == "" || event.Category == "" || event.Version < 1 || event.EntryCount < 0 {
		return ErrInvalidAuthorizationAuditAdapter
	}
	_, err := adapter.appender.Execute(ctx, auditapp.AppendCommand{
		Scope:  auditdomain.ScopeSystem,
		Actor:  auditdomain.Actor{Type: auditdomain.ActorUser, ID: event.ActorUserID},
		Action: "localization.catalog.updated", RequestID: event.RequestID,
		Data: map[string]any{
			"locale":      event.Locale,
			"category":    event.Category,
			"version":     event.Version,
			"entry_count": event.EntryCount,
		},
	})
	return err
}

var _ authorizationapp.RoleAssignmentAuditor = authorizationAuditAdapter{}
var _ authorizationapp.GrantRolePermissionAuditor = authorizationAuditAdapter{}
var _ authorizationapp.RoleManagementAuditor = authorizationAuditAdapter{}
var _ identityapp.AdminUserAuditor = authorizationAuditAdapter{}
var _ accountsapp.MembershipAuditor = authorizationAuditAdapter{}
var _ localizationapp.LocaleManagementAuditor = authorizationAuditAdapter{}
var _ catalogapp.CatalogAuditor = authorizationAuditAdapter{}
