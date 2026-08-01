package domain

import (
	"errors"
	"strings"
	"time"
)

type Kind string

const (
	KindPage Kind = "page"
	KindPost Kind = "post"
)

var ErrInvalidContentItem = errors.New("publishing: invalid content item")

type ContentItem struct {
	ID         string
	AccountID  string
	Kind       Kind
	ContentKey string
	Status     Status
	Version    int64
	DeletedAt  *time.Time
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

type ContentItemInput struct {
	ID         string
	AccountID  string
	Kind       Kind
	ContentKey string
	Status     Status
	Version    int64
	DeletedAt  *time.Time
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

func NewContentItem(input ContentItemInput) (ContentItem, error) {
	contentKey, err := NormalizeContentKey(input.ContentKey)
	if err != nil ||
		!validIdentity(input.ID) ||
		!validIdentity(input.AccountID) ||
		(input.Kind != KindPage && input.Kind != KindPost) ||
		input.Version <= 0 {
		return ContentItem{}, ErrInvalidContentItem
	}
	status := input.Status
	if status == "" {
		status = StatusDraft
	}
	if ValidateStatus(status) != nil ||
		(!input.CreatedAt.IsZero() && !input.UpdatedAt.IsZero() &&
			input.UpdatedAt.Before(input.CreatedAt)) {
		return ContentItem{}, ErrInvalidContentItem
	}
	return ContentItem{
		ID:         strings.TrimSpace(input.ID),
		AccountID:  strings.TrimSpace(input.AccountID),
		Kind:       input.Kind,
		ContentKey: contentKey,
		Status:     status,
		Version:    input.Version,
		DeletedAt:  cloneTime(input.DeletedAt),
		CreatedAt:  input.CreatedAt.UTC(),
		UpdatedAt:  input.UpdatedAt.UTC(),
	}, nil
}
