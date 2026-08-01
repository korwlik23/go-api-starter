package application

import (
	"context"
	"errors"
	"strings"

	auditmodule "github.com/korwlik23/go-api-starter/internal/modules/audit"
	auditports "github.com/korwlik23/go-api-starter/internal/modules/audit/ports"
	authdomain "github.com/korwlik23/go-api-starter/internal/modules/authorization/domain"
)

var (
	ErrInvalidAuditQuery = errors.New("invalid audit query")
	ErrAuditQueryDenied  = errors.New("audit query denied")
)

type AuditQueryAuthorizer interface {
	Allowed(context.Context, string, authdomain.PermissionKey) (bool, error)
}

type ListSystemEventsCommand struct {
	ActorUserID string
	Action      string
	TargetActor string
	Cursor      *auditports.Cursor
	Limit       int
}

type ListSystemEvents struct {
	authorizer AuditQueryAuthorizer
	repository auditports.SystemEventQuery
	permission authdomain.PermissionKey
}

func NewListSystemEvents(authorizer AuditQueryAuthorizer, repository auditports.SystemEventQuery) (*ListSystemEvents, error) {
	permission, err := authdomain.ParsePermissionKey(auditmodule.PermissionEventsReadSystem)
	if err != nil || authorizer == nil || repository == nil {
		return nil, ErrInvalidAuditQuery
	}
	return &ListSystemEvents{authorizer: authorizer, repository: repository, permission: permission}, nil
}

func (query *ListSystemEvents) Execute(ctx context.Context, command ListSystemEventsCommand) (auditports.EventPage, error) {
	if query == nil || ctx == nil || strings.TrimSpace(command.ActorUserID) == "" || command.Limit < 1 || command.Limit > 200 || strings.TrimSpace(command.Action) != command.Action || strings.TrimSpace(command.TargetActor) != command.TargetActor {
		return auditports.EventPage{}, ErrInvalidAuditQuery
	}
	allowed, err := query.authorizer.Allowed(ctx, command.ActorUserID, query.permission)
	if err != nil {
		return auditports.EventPage{}, err
	}
	if !allowed {
		return auditports.EventPage{}, ErrAuditQueryDenied
	}
	return query.repository.ListSystemFiltered(ctx, auditports.SystemEventFilter{Cursor: command.Cursor, Limit: command.Limit, Action: command.Action, ActorID: command.TargetActor})
}
