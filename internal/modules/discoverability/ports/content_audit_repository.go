package ports

import (
	"context"
	"errors"
	"regexp"
	"time"

	discoverabilitydomain "github.com/korwlik23/go-api-starter/internal/modules/discoverability/domain"
)

var (
	ErrInvalidContentAuditRepository = errors.New("invalid content audit repository")
	ErrInvalidContentAuditQuery      = errors.New("invalid content audit query")
	ErrContentAuditNotFound          = errors.New("content audit not found")
	ErrContentAuditConflict          = errors.New("content audit conflict")
)

var contentAuditIDPattern = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

type ContentAuditCursor struct {
	UpdatedAt time.Time
	ID        string
}

type ContentAuditListQuery struct {
	AccountID     string
	ContentID     string
	TranslationID string
	LocaleID      string
	Status        *discoverabilitydomain.AuditStatus
	After         *ContentAuditCursor
	Limit         int
}

type ContentAuditListPage struct {
	Items []discoverabilitydomain.ContentAudit
	Next  *ContentAuditCursor
}

type SaveContentAuditCommand struct {
	Audit discoverabilitydomain.ContentAudit
}

type ContentAuditRepository interface {
	Save(context.Context, SaveContentAuditCommand) (discoverabilitydomain.ContentAudit, error)
	Get(context.Context, string, string) (discoverabilitydomain.ContentAudit, error)
	List(context.Context, ContentAuditListQuery) (ContentAuditListPage, error)
}

func ValidateContentAuditListQuery(query ContentAuditListQuery) error {
	if !contentAuditIDPattern.MatchString(query.AccountID) || query.Limit < 1 || query.Limit > 100 {
		return ErrInvalidContentAuditQuery
	}
	for _, value := range []string{query.ContentID, query.TranslationID, query.LocaleID} {
		if value != "" && !contentAuditIDPattern.MatchString(value) {
			return ErrInvalidContentAuditQuery
		}
	}
	if query.Status != nil && *query.Status != discoverabilitydomain.AuditStatusPass && *query.Status != discoverabilitydomain.AuditStatusWarning && *query.Status != discoverabilitydomain.AuditStatusError {
		return ErrInvalidContentAuditQuery
	}
	if query.After != nil && (query.After.UpdatedAt.IsZero() || !contentAuditIDPattern.MatchString(query.After.ID)) {
		return ErrInvalidContentAuditQuery
	}
	return nil
}

func ValidateContentAuditIdentity(accountID, auditID string) error {
	if !contentAuditIDPattern.MatchString(accountID) || !contentAuditIDPattern.MatchString(auditID) {
		return ErrInvalidContentAuditQuery
	}
	return nil
}
