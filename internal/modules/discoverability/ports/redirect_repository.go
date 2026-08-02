package ports

import (
	"context"
	"errors"
	"regexp"
	"time"

	discoverabilitydomain "github.com/korwlik23/go-api-starter/internal/modules/discoverability/domain"
)

var (
	ErrInvalidRedirectRepository = errors.New("invalid redirect repository")
	ErrInvalidRedirectQuery      = errors.New("invalid redirect query")
	ErrRedirectNotFound          = errors.New("redirect not found")
	ErrRedirectConflict          = errors.New("redirect version conflict")
	ErrSEODefaultsNotFound       = errors.New("SEO defaults not found")
	ErrSEODefaultsConflict       = errors.New("SEO defaults version conflict")
)

var redirectIDPattern = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

type RedirectCursor struct {
	UpdatedAt time.Time
	ID        string
}

type RedirectListQuery struct {
	AccountID string
	LocaleID  string
	Enabled   *bool
	After     *RedirectCursor
	Limit     int
}

type RedirectListPage struct {
	Items []discoverabilitydomain.Redirect
	Next  *RedirectCursor
}

type CreateRedirectCommand struct {
	Redirect discoverabilitydomain.Redirect
}

type UpdateRedirectCommand struct {
	AccountID       string
	RedirectID      string
	ExpectedVersion int64
	Redirect        discoverabilitydomain.Redirect
}

type DeleteRedirectCommand struct {
	AccountID       string
	RedirectID      string
	ExpectedVersion int64
	DeletedAt       time.Time
}

type UpsertSEODefaultsCommand struct {
	Defaults        discoverabilitydomain.SEODefaults
	ExpectedVersion int64
}

type RedirectRepository interface {
	List(context.Context, RedirectListQuery) (RedirectListPage, error)
	Get(context.Context, string, string) (discoverabilitydomain.Redirect, error)
	Create(context.Context, CreateRedirectCommand) (discoverabilitydomain.Redirect, error)
	Update(context.Context, UpdateRedirectCommand) (discoverabilitydomain.Redirect, error)
	Delete(context.Context, DeleteRedirectCommand) error
}

type SEODefaultsRepository interface {
	GetSEODefaults(context.Context, string, string) (discoverabilitydomain.SEODefaults, error)
	UpsertSEODefaults(context.Context, UpsertSEODefaultsCommand) (discoverabilitydomain.SEODefaults, error)
}

func ValidateRedirectListQuery(query RedirectListQuery) error {
	if !redirectIDPattern.MatchString(query.AccountID) || query.Limit < 1 || query.Limit > 100 {
		return ErrInvalidRedirectQuery
	}
	if query.LocaleID != "" && !redirectIDPattern.MatchString(query.LocaleID) {
		return ErrInvalidRedirectQuery
	}
	if query.After != nil && (query.After.UpdatedAt.IsZero() || !redirectIDPattern.MatchString(query.After.ID)) {
		return ErrInvalidRedirectQuery
	}
	return nil
}

func ValidateRedirectIdentity(accountID, redirectID string) error {
	if !redirectIDPattern.MatchString(accountID) || !redirectIDPattern.MatchString(redirectID) {
		return ErrInvalidRedirectQuery
	}
	return nil
}
