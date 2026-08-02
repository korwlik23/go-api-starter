package domain

import (
	"errors"
	"regexp"
	"strings"
	"time"
)

type StatusCode int

const (
	StatusMovedPermanently StatusCode = 301
	StatusFound            StatusCode = 302
	StatusTemporary        StatusCode = 307
	StatusPermanent        StatusCode = 308
)

var (
	ErrInvalidRedirect = errors.New("discoverability: invalid redirect")
	ErrRedirectLoop    = errors.New("discoverability: redirect loop")
)

type Redirect struct {
	ID              string
	AccountID       string
	LocaleID        string
	SourcePath      string
	DestinationPath string
	StatusCode      StatusCode
	Enabled         bool
	Version         int64
	DeletedAt       *time.Time
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

type RedirectInput struct {
	ID              string
	AccountID       string
	LocaleID        string
	SourcePath      string
	DestinationPath string
	StatusCode      StatusCode
	Enabled         bool
	Version         int64
	DeletedAt       *time.Time
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

var redirectPathPattern = regexp.MustCompile(`^/[a-z0-9][a-z0-9/_-]*$`)
var redirectIDPattern = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

func NewRedirect(input RedirectInput) (Redirect, error) {
	source, err := NormalizeRedirectPath(input.SourcePath)
	if err != nil {
		return Redirect{}, ErrInvalidRedirect
	}
	destination, err := NormalizeRedirectPath(input.DestinationPath)
	if err != nil {
		return Redirect{}, ErrInvalidRedirect
	}
	if source == destination {
		return Redirect{}, ErrRedirectLoop
	}
	if !redirectIDPattern.MatchString(input.ID) || !redirectIDPattern.MatchString(input.AccountID) || !redirectIDPattern.MatchString(input.LocaleID) || input.Version < 1 || !validStatusCode(input.StatusCode) || (!input.CreatedAt.IsZero() && !input.UpdatedAt.IsZero() && input.UpdatedAt.Before(input.CreatedAt)) {
		return Redirect{}, ErrInvalidRedirect
	}
	return Redirect{ID: input.ID, AccountID: input.AccountID, LocaleID: input.LocaleID, SourcePath: source, DestinationPath: destination, StatusCode: input.StatusCode, Enabled: input.Enabled, Version: input.Version, DeletedAt: cloneTime(input.DeletedAt), CreatedAt: input.CreatedAt.UTC(), UpdatedAt: input.UpdatedAt.UTC()}, nil
}

func NormalizeRedirectPath(value string) (string, error) {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "/" {
		return value, nil
	}
	if len(value) == 0 || len(value) > 512 || strings.Contains(value, "//") || strings.ContainsAny(value, "?#") || !redirectPathPattern.MatchString(value) {
		return "", ErrInvalidRedirect
	}
	return value, nil
}

func validStatusCode(code StatusCode) bool {
	return code == StatusMovedPermanently || code == StatusFound || code == StatusTemporary || code == StatusPermanent
}

func cloneTime(value *time.Time) *time.Time {
	if value == nil {
		return nil
	}
	cloned := value.UTC()
	return &cloned
}
