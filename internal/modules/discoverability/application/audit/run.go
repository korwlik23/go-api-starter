package audit

import (
	"context"
	"errors"
	"strings"
	"time"

	discoverabilitydomain "github.com/korwlik23/go-api-starter/internal/modules/discoverability/domain"
	"github.com/korwlik23/go-api-starter/internal/modules/discoverability/ports"
	publishingports "github.com/korwlik23/go-api-starter/internal/modules/publishing/ports"
)

var (
	ErrInvalidAuditService = errors.New("discoverability: invalid audit service")
	ErrInvalidAuditSubject = errors.New("discoverability: invalid audit subject")
)

type Dependencies struct {
	Content ContentAuditContentRepository
	Audits  ports.ContentAuditRepository
	NewID   func() (string, error)
	Now     func() time.Time
}

type Service struct{ dependencies Dependencies }

func NewService(dependencies Dependencies) (*Service, error) {
	if dependencies.Content == nil || dependencies.Audits == nil || dependencies.NewID == nil || dependencies.Now == nil {
		return nil, ErrInvalidAuditService
	}
	return &Service{dependencies: dependencies}, nil
}

type RunInput struct {
	AccountID string
	ContentID string
	LocaleID  string
}

type ListInput struct {
	AccountID     string
	ContentID     string
	TranslationID string
	LocaleID      string
	Status        *discoverabilitydomain.AuditStatus
	After         *ports.ContentAuditCursor
	Limit         int
}

func (service *Service) Run(ctx context.Context, input RunInput) (discoverabilitydomain.ContentAudit, error) {
	if service == nil || ctx == nil || validateIdentity(input.AccountID, input.ContentID, input.LocaleID) != nil {
		return discoverabilitydomain.ContentAudit{}, ErrInvalidAuditSubject
	}
	detail, err := service.dependencies.Content.Get(ctx, input.AccountID, input.ContentID, input.LocaleID)
	if err != nil {
		return discoverabilitydomain.ContentAudit{}, err
	}
	if strings.TrimSpace(detail.AccountID) != input.AccountID || strings.TrimSpace(detail.ContentID) != input.ContentID || strings.TrimSpace(detail.LocaleID) != input.LocaleID || strings.TrimSpace(detail.TranslationID) == "" {
		return discoverabilitydomain.ContentAudit{}, ErrInvalidAuditSubject
	}
	report, err := discoverabilitydomain.RunContentAudit(discoverabilitydomain.AuditSubject{
		ID: detail.TranslationID, AccountID: detail.AccountID, ContentID: detail.ContentID, LocaleID: detail.LocaleID,
		Kind: detail.Kind, Version: detail.ContentVersion, TranslationVersion: detail.TranslationVersion,
		Title: detail.Title, Excerpt: detail.Excerpt, Blocks: detail.Blocks, SEO: detail.SEO, GEO: detail.GEO, AEO: detail.AEO,
	})
	if err != nil {
		return discoverabilitydomain.ContentAudit{}, err
	}
	id, err := service.dependencies.NewID()
	if err != nil {
		return discoverabilitydomain.ContentAudit{}, err
	}
	now := service.dependencies.Now().UTC()
	if now.IsZero() {
		return discoverabilitydomain.ContentAudit{}, ErrInvalidAuditSubject
	}
	audit, err := discoverabilitydomain.NewContentAudit(discoverabilitydomain.ContentAuditInput{
		ID: id, AccountID: detail.AccountID, ContentID: detail.ContentID, TranslationID: detail.TranslationID, LocaleID: detail.LocaleID,
		ContentVersion: detail.ContentVersion, TranslationVersion: detail.TranslationVersion, Status: report.Status, Checks: report.Checks,
		CreatedAt: now, UpdatedAt: now,
	})
	if err != nil {
		return discoverabilitydomain.ContentAudit{}, err
	}
	return service.dependencies.Audits.Save(ctx, ports.SaveContentAuditCommand{Audit: audit})
}

func (service *Service) Get(ctx context.Context, accountID, auditID string) (discoverabilitydomain.ContentAudit, error) {
	if service == nil || ctx == nil || ports.ValidateContentAuditIdentity(accountID, auditID) != nil {
		return discoverabilitydomain.ContentAudit{}, ports.ErrInvalidContentAuditQuery
	}
	return service.dependencies.Audits.Get(ctx, accountID, auditID)
}

func (service *Service) List(ctx context.Context, input ListInput) (ports.ContentAuditListPage, error) {
	if service == nil || ctx == nil {
		return ports.ContentAuditListPage{}, ports.ErrInvalidContentAuditQuery
	}
	query := ports.ContentAuditListQuery{AccountID: input.AccountID, ContentID: input.ContentID, TranslationID: input.TranslationID, LocaleID: input.LocaleID, Status: input.Status, After: input.After, Limit: input.Limit}
	if err := ports.ValidateContentAuditListQuery(query); err != nil {
		return ports.ContentAuditListPage{}, err
	}
	return service.dependencies.Audits.List(ctx, query)
}

func validateIdentity(accountID, contentID, localeID string) error {
	return ports.ValidateContentAuditListQuery(ports.ContentAuditListQuery{AccountID: accountID, ContentID: contentID, LocaleID: localeID, Limit: 1})
}

// ContentAuditContentRepository keeps the audit application layer independent
// from the publishing module's concrete repository implementation.
type ContentAuditContentRepository interface {
	Get(context.Context, string, string, string) (publishingports.ContentDetail, error)
}
