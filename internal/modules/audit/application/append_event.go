package application

import (
	"context"
	"errors"
	"time"

	auditdomain "github.com/korwlik23/go-api-starter/internal/modules/audit/domain"
)

var ErrInvalidAppendEvent = errors.New("audit: invalid append dependencies")

type eventAppender interface {
	Append(context.Context, auditdomain.Event) error
}

type AppendDependencies struct {
	Repository eventAppender
	NewID      func() (string, error)
	Now        func() time.Time
}

type AppendCommand struct {
	Scope     auditdomain.Scope
	AccountID string
	Actor     auditdomain.Actor
	Action    string
	RequestID string
	Data      map[string]any
}

type AppendEvent struct {
	dependencies AppendDependencies
}

func NewAppendEvent(
	dependencies AppendDependencies,
) (*AppendEvent, error) {
	if dependencies.Repository == nil ||
		dependencies.NewID == nil ||
		dependencies.Now == nil {
		return nil, ErrInvalidAppendEvent
	}
	return &AppendEvent{dependencies: dependencies}, nil
}

func (appender *AppendEvent) Execute(
	ctx context.Context,
	command AppendCommand,
) (auditdomain.Event, error) {
	if appender == nil || ctx == nil {
		return auditdomain.Event{}, ErrInvalidAppendEvent
	}
	eventID, err := appender.dependencies.NewID()
	if err != nil {
		return auditdomain.Event{}, err
	}
	operationID, err := appender.dependencies.NewID()
	if err != nil {
		return auditdomain.Event{}, err
	}
	event, err := auditdomain.NewEvent(auditdomain.EventInput{
		ID:          eventID,
		Scope:       command.Scope,
		AccountID:   command.AccountID,
		Actor:       command.Actor,
		Action:      command.Action,
		OperationID: operationID,
		RequestID:   command.RequestID,
		Data:        command.Data,
		OccurredAt:  appender.dependencies.Now(),
	})
	if err != nil {
		return auditdomain.Event{}, err
	}
	if err := appender.dependencies.Repository.Append(ctx, event); err != nil {
		return auditdomain.Event{}, err
	}
	return event, nil
}
