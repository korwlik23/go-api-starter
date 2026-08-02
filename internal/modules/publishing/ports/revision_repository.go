package ports

import (
	"context"
	"encoding/json"
	"errors"
	"regexp"
	"time"

	publishingdomain "github.com/korwlik23/go-api-starter/internal/modules/publishing/domain"
)

var (
	ErrInvalidRevisionRepository = errors.New("invalid revision repository")
	ErrInvalidRevisionQuery      = errors.New("invalid revision query")
	ErrRevisionNotFound          = errors.New("revision not found")
	ErrRevisionConflict          = errors.New("revision version conflict")
)

type RevisionCursor struct {
	CreatedAt time.Time
	ID        string
}

type RevisionListQuery struct {
	AccountID     string
	ContentID     string
	TranslationID string
	After         *RevisionCursor
	Limit         int
}

type RevisionView struct {
	AccountID string
	Revision  publishingdomain.Revision
	Snapshot  json.RawMessage
}

type CreateRevisionCommand struct {
	AccountID string
	Revision  publishingdomain.Revision
	Snapshot  json.RawMessage
}

type RestoreContentCommand struct {
	AccountID       string
	ContentID       string
	TranslationID   string
	ExpectedVersion int64
	Detail          ContentDetail
	UpdatedAt       time.Time
}

type RevisionPage struct {
	Items []RevisionView
	Next  *RevisionCursor
}

type RevisionRepository interface {
	Create(context.Context, CreateRevisionCommand) error
	List(context.Context, RevisionListQuery) (RevisionPage, error)
	Get(context.Context, string, string, string) (RevisionView, error)
	RestoreContent(context.Context, RestoreContentCommand) (ContentDetail, error)
}

var revisionIDPattern = regexp.MustCompile(
	`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`,
)

func ValidateRevisionListQuery(query RevisionListQuery) error {
	if !revisionIDPattern.MatchString(query.AccountID) ||
		!revisionIDPattern.MatchString(query.ContentID) ||
		!revisionIDPattern.MatchString(query.TranslationID) ||
		query.Limit < 1 || query.Limit > 100 {
		return ErrInvalidRevisionQuery
	}
	if query.After != nil &&
		(query.After.CreatedAt.IsZero() || !revisionIDPattern.MatchString(query.After.ID)) {
		return ErrInvalidRevisionQuery
	}
	return nil
}
