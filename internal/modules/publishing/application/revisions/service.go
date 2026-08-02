package revisions

import (
	"context"
	"errors"
	"time"

	"github.com/korwlik23/go-api-starter/internal/modules/publishing/ports"
	"github.com/korwlik23/go-api-starter/internal/platform/database"
)

var ErrInvalidRevisionService = errors.New("publishing: invalid revision service")

type RevisionAuditEvent struct {
	ActorUserID   string
	AccountID     string
	RequestID     string
	Action        string
	ContentID     string
	TranslationID string
	RevisionID    string
	Version       int64
	Data          map[string]any
}

type RevisionAuditor interface {
	RecordRevisionChange(context.Context, RevisionAuditEvent) error
}

type RevisionDependencies struct {
	Repository   ports.RevisionRepository
	Content      ports.ContentRepository
	Transactions database.Transactor
	Auditor      RevisionAuditor
	NewID        func() (string, error)
	Now          func() time.Time
}

type Service struct {
	dependencies RevisionDependencies
}

func NewService(dependencies RevisionDependencies) (*Service, error) {
	if dependencies.Repository == nil || dependencies.Content == nil ||
		dependencies.NewID == nil || dependencies.Now == nil {
		return nil, ErrInvalidRevisionService
	}
	return &Service{dependencies: dependencies}, nil
}

type CreateInput struct {
	AccountID     string
	ActorUserID   string
	RequestID     string
	ContentID     string
	TranslationID string
	ChangeSummary string
	SourceVersion int64
	Snapshot      Snapshot
}

type ListInput struct {
	AccountID     string
	ContentID     string
	TranslationID string
	After         *ports.RevisionCursor
	Limit         int
}

type CompareInput struct {
	AccountID        string
	ContentID        string
	BaseRevisionID   string
	TargetRevisionID string
}

type FieldChange struct {
	Field string
	From  any
	To    any
}

type CompareResult struct {
	Base    ports.RevisionView
	Target  ports.RevisionView
	Changes []FieldChange
}

type RollbackInput struct {
	AccountID       string
	ActorUserID     string
	RequestID       string
	ContentID       string
	TranslationID   string
	LocaleID        string
	RevisionID      string
	ExpectedVersion int64
	ChangeSummary   string
}

type RollbackResult struct {
	Revision ports.RevisionView
	Content  ports.ContentDetail
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
