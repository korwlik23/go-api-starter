package schedules

import (
	"context"
	"errors"
	"time"

	publishingdomain "github.com/korwlik23/go-api-starter/internal/modules/publishing/domain"
	"github.com/korwlik23/go-api-starter/internal/modules/publishing/ports"
	"github.com/korwlik23/go-api-starter/internal/platform/database"
)

var (
	ErrInvalidScheduleService = errors.New("publishing: invalid schedule service")
	ErrInvalidScheduleCommand = errors.New("publishing: invalid schedule command")
)

type ScheduleAuditEvent struct {
	ActorUserID   string
	AccountID     string
	RequestID     string
	Action        string
	ScheduleID    string
	TranslationID string
	State         publishingdomain.ScheduleState
	Attempts      int
}

type ScheduleAuditor interface {
	RecordScheduleChange(context.Context, ScheduleAuditEvent) error
}

type PublicationExecutor interface {
	Execute(context.Context, publishingdomain.PublicationSchedule) error
}

type Dependencies struct {
	Repository    ports.ScheduleRepository
	Transactions  database.Transactor
	Auditor       ScheduleAuditor
	Executor      PublicationExecutor
	NewID         func() (string, error)
	Now           func() time.Time
	LeaseDuration time.Duration
	MaxAttempts   int
}

type Service struct{ dependencies Dependencies }

func NewService(dependencies Dependencies) (*Service, error) {
	if dependencies.Repository == nil || dependencies.Auditor == nil ||
		dependencies.Executor == nil || dependencies.NewID == nil || dependencies.Now == nil ||
		dependencies.LeaseDuration <= 0 || dependencies.MaxAttempts < 1 {
		return nil, ErrInvalidScheduleService
	}
	return &Service{dependencies: dependencies}, nil
}

type CreateInput struct {
	AccountID       string
	ActorUserID     string
	RequestID       string
	TranslationID   string
	PublishAt       time.Time
	UnpublishAt     *time.Time
	ExpectedVersion int64
	IdempotencyKey  string
}

type CancelInput struct {
	AccountID   string
	ActorUserID string
	RequestID   string
	ScheduleID  string
}

type RunDueInput struct{ Limit int }

type ListInput struct {
	AccountID string
	Limit     int
}

type RunDueResult struct {
	Claimed   int
	Succeeded int
	Retried   int
	Failed    int
}

func (service *Service) runMutation(ctx context.Context, operation func(context.Context) error) error {
	if service.dependencies.Transactions == nil {
		return operation(ctx)
	}
	return service.dependencies.Transactions.WithinTransaction(ctx, operation)
}
