package redirects

import (
	"context"
	"errors"
	"time"

	discoverabilitydomain "github.com/korwlik23/go-api-starter/internal/modules/discoverability/domain"
	"github.com/korwlik23/go-api-starter/internal/modules/discoverability/ports"
	"github.com/korwlik23/go-api-starter/internal/platform/database"
)

var ErrInvalidRedirectMutation = errors.New("discoverability: invalid redirect mutation")

type RedirectAuditEvent struct {
	ActorUserID string
	AccountID   string
	RequestID   string
	Action      string
	RedirectID  string
	Version     int64
	Data        map[string]any
}

type RedirectAuditor interface {
	RecordRedirectChange(context.Context, RedirectAuditEvent) error
}

type Dependencies struct {
	Repository   ports.RedirectRepository
	Transactions database.Transactor
	Auditor      RedirectAuditor
	NewID        func() (string, error)
	Now          func() time.Time
}

type Service struct{ dependencies Dependencies }

func NewService(dependencies Dependencies) (*Service, error) {
	if dependencies.Repository == nil || dependencies.Auditor == nil || dependencies.NewID == nil || dependencies.Now == nil {
		return nil, ErrInvalidRedirectMutation
	}
	return &Service{dependencies: dependencies}, nil
}

type CreateInput struct {
	AccountID, ActorUserID, RequestID, LocaleID, SourcePath, DestinationPath string
	StatusCode                                                               discoverabilitydomain.StatusCode
	Enabled                                                                  bool
}
type UpdateInput struct {
	AccountID, ActorUserID, RequestID, RedirectID, LocaleID, SourcePath, DestinationPath string
	ExpectedVersion                                                                      int64
	StatusCode                                                                           discoverabilitydomain.StatusCode
	Enabled                                                                              bool
}
type DeleteInput struct {
	AccountID, ActorUserID, RequestID, RedirectID string
	ExpectedVersion                               int64
}
type ListInput struct {
	AccountID, LocaleID string
	Enabled             *bool
	After               *ports.RedirectCursor
	Limit               int
}

func (service *Service) runMutation(ctx context.Context, operation func(context.Context) error) error {
	if service.dependencies.Transactions == nil {
		return operation(ctx)
	}
	return service.dependencies.Transactions.WithinTransaction(ctx, operation)
}
