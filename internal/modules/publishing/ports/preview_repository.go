package ports

import (
	"context"
	"errors"
	"regexp"
	"time"
)

var (
	ErrInvalidPreviewRepository = errors.New("invalid preview repository")
	ErrPreviewNotFound          = errors.New("preview not found")
	ErrPreviewConflict          = errors.New("preview token conflict")
)

var previewIDPattern = regexp.MustCompile(
	`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`,
)

type PreviewToken struct {
	ID             string
	UserID         string
	AccountID      string
	ContentID      string
	TranslationID  string
	RevisionID     string
	TokenHash      [32]byte
	HashVersion    int16
	HashKeyVersion int64
	Audience       string
	ExpiresAt      time.Time
	CreatedAt      time.Time
}

type PreviewRepository interface {
	Create(context.Context, PreviewToken) error
	Find(context.Context, string) (PreviewToken, error)
	Consume(context.Context, string, [32]byte, time.Time) error
}

func ValidatePreviewToken(token PreviewToken) error {
	for _, value := range []string{
		token.ID, token.UserID, token.AccountID, token.ContentID,
		token.TranslationID, token.RevisionID,
	} {
		if !previewIDPattern.MatchString(value) {
			return ErrInvalidPreviewRepository
		}
	}
	if token.HashVersion < 1 || token.HashKeyVersion < 1 || token.TokenHash == [32]byte{} ||
		token.Audience == "" || token.CreatedAt.IsZero() || !token.ExpiresAt.After(token.CreatedAt) {
		return ErrInvalidPreviewRepository
	}
	return nil
}
