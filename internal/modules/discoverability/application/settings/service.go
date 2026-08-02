package settings

import (
	"context"
	"errors"
	"strings"
	"time"

	discoverabilitydomain "github.com/korwlik23/go-api-starter/internal/modules/discoverability/domain"
	"github.com/korwlik23/go-api-starter/internal/modules/discoverability/ports"
	"github.com/korwlik23/go-api-starter/internal/platform/database"
)

var ErrInvalidSEOSettings = errors.New("discoverability: invalid SEO settings")

type SEOAuditEvent struct {
	ActorUserID, AccountID, RequestID, Action, LocaleID string
	Version                                             int64
	Data                                                map[string]any
}
type SEOAuditor interface {
	RecordSEOChange(context.Context, SEOAuditEvent) error
}
type Dependencies struct {
	Repository   ports.SEODefaultsRepository
	Transactions database.Transactor
	Auditor      SEOAuditor
	NewID        func() (string, error)
	Now          func() time.Time
}
type Service struct{ dependencies Dependencies }

func NewService(dependencies Dependencies) (*Service, error) {
	if dependencies.Repository == nil || dependencies.Auditor == nil || dependencies.NewID == nil || dependencies.Now == nil {
		return nil, ErrInvalidSEOSettings
	}
	return &Service{dependencies: dependencies}, nil
}

type UpdateInput struct {
	AccountID, ActorUserID, RequestID, LocaleID, Title, Description, CanonicalBase, Robots string
	ExpectedVersion                                                                        int64
}

func (service *Service) runMutation(ctx context.Context, operation func(context.Context) error) error {
	if service.dependencies.Transactions == nil {
		return operation(ctx)
	}
	return service.dependencies.Transactions.WithinTransaction(ctx, operation)
}

func (service *Service) Get(ctx context.Context, accountID, localeID string) (discoverabilitydomain.SEODefaults, error) {
	if service == nil || ctx == nil || !validSEOIdentity(accountID, localeID) {
		return discoverabilitydomain.SEODefaults{}, ErrInvalidSEOSettings
	}
	return service.dependencies.Repository.GetSEODefaults(ctx, accountID, localeID)
}

func (service *Service) Update(ctx context.Context, input UpdateInput) (result discoverabilitydomain.SEODefaults, resultErr error) {
	if service == nil || ctx == nil || strings.TrimSpace(input.ActorUserID) == "" || strings.TrimSpace(input.RequestID) == "" || input.ExpectedVersion < 0 || !validSEOIdentity(input.AccountID, input.LocaleID) {
		return discoverabilitydomain.SEODefaults{}, ErrInvalidSEOSettings
	}
	now := service.dependencies.Now().UTC()
	if now.IsZero() {
		return discoverabilitydomain.SEODefaults{}, ErrInvalidSEOSettings
	}
	id, err := service.dependencies.NewID()
	if err != nil {
		return discoverabilitydomain.SEODefaults{}, err
	}
	defaults, err := discoverabilitydomain.NewSEODefaults(discoverabilitydomain.SEODefaultsInput{ID: id, AccountID: input.AccountID, LocaleID: input.LocaleID, Title: input.Title, Description: input.Description, CanonicalBase: input.CanonicalBase, Robots: input.Robots, Version: input.ExpectedVersion + 1, CreatedAt: now, UpdatedAt: now})
	if err != nil {
		return discoverabilitydomain.SEODefaults{}, ErrInvalidSEOSettings
	}
	operation := func(operationContext context.Context) error {
		result, err = service.dependencies.Repository.UpsertSEODefaults(operationContext, ports.UpsertSEODefaultsCommand{Defaults: defaults, ExpectedVersion: input.ExpectedVersion})
		if err != nil {
			return err
		}
		return service.dependencies.Auditor.RecordSEOChange(operationContext, SEOAuditEvent{ActorUserID: input.ActorUserID, AccountID: input.AccountID, RequestID: input.RequestID, Action: "discoverability.seo.updated", LocaleID: input.LocaleID, Version: defaults.Version, Data: map[string]any{"robots": defaults.Robots}})
	}
	resultErr = service.runMutation(ctx, operation)
	return result, resultErr
}

func validSEOIdentity(accountID, localeID string) bool {
	return ports.ValidateRedirectListQuery(ports.RedirectListQuery{AccountID: accountID, LocaleID: localeID, Limit: 1}) == nil
}
