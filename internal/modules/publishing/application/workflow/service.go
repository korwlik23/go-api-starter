package workflow

import (
	"context"
	"errors"
	"time"

	authdomain "github.com/korwlik23/go-api-starter/internal/modules/authorization/domain"
	"github.com/korwlik23/go-api-starter/internal/modules/publishing/ports"
	"github.com/korwlik23/go-api-starter/internal/platform/database"
)

var (
	ErrInvalidWorkflowService = errors.New("publishing: invalid workflow service")
	ErrWorkflowDenied         = errors.New("publishing: workflow permission denied")
	ErrInvalidWorkflowCommand = errors.New("publishing: invalid workflow command")
)

type WorkflowAuthorizer interface {
	Allowed(context.Context, string, string, authdomain.PermissionKey, string) (bool, error)
}

type WorkflowAuditEvent struct {
	ActorUserID   string
	AccountID     string
	RequestID     string
	Action        string
	ContentID     string
	TranslationID string
	FromStatus    string
	ToStatus      string
	Version       int64
}

type WorkflowAuditor interface {
	RecordWorkflowChange(context.Context, WorkflowAuditEvent) error
}

type Dependencies struct {
	Repository   ports.WorkflowRepository
	Authorizer   WorkflowAuthorizer
	Transactions database.Transactor
	Auditor      WorkflowAuditor
	NewID        func() (string, error)
	Now          func() time.Time
}

type Service struct {
	dependencies Dependencies
}

func NewService(dependencies Dependencies) (*Service, error) {
	if dependencies.Repository == nil || dependencies.Authorizer == nil ||
		dependencies.Auditor == nil || dependencies.NewID == nil || dependencies.Now == nil {
		return nil, ErrInvalidWorkflowService
	}
	return &Service{dependencies: dependencies}, nil
}

type TransitionInput struct {
	AccountID       string
	ActorUserID     string
	RequestID       string
	ContentID       string
	TranslationID   string
	LocaleID        string
	ExpectedVersion int64
	Action          string
}

func (service *Service) runMutation(
	ctx context.Context,
	operation func(context.Context) error,
) error {
	if service.dependencies.Transactions == nil {
		return operation(ctx)
	}
	return service.dependencies.Transactions.WithinTransaction(ctx, operation)
}
