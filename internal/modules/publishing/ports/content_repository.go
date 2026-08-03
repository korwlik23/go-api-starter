package ports

import (
	"context"
	"errors"
	"regexp"
	"time"

	publishingdomain "github.com/korwlik23/go-api-starter/internal/modules/publishing/domain"
)

var (
	ErrInvalidContentRepository = errors.New("invalid content repository")
	ErrInvalidContentQuery      = errors.New("invalid content query")
	ErrContentNotFound          = errors.New("content not found")
	ErrContentConflict          = errors.New("content version conflict")
)

var contentIDPattern = regexp.MustCompile(
	`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`,
)

type ContentCursor struct {
	UpdatedAt time.Time
	ID        string
}

type ContentListQuery struct {
	AccountID string
	LocaleID  string
	Kind      *publishingdomain.Kind
	Status    []publishingdomain.Status
	After     *ContentCursor
	Limit     int
}

type ContentListItem struct {
	ContentID          string
	AccountID          string
	Kind               publishingdomain.Kind
	ContentKey         string
	ContentStatus      publishingdomain.Status
	ContentVersion     int64
	TranslationID      string
	LocaleID           string
	Slug               string
	Path               string
	Title              string
	TranslationStatus  publishingdomain.Status
	TranslationVersion int64
	UpdatedAt          time.Time
}

type ContentListPage struct {
	Items []ContentListItem
	Next  *ContentCursor
}

type ContentDetail struct {
	ContentID          string
	TranslationID      string
	AccountID          string
	LocaleID           string
	Kind               publishingdomain.Kind
	ContentKey         string
	ContentStatus      publishingdomain.Status
	ContentVersion     int64
	TranslationStatus  publishingdomain.Status
	TranslationVersion int64
	Slug               string
	Path               string
	Title              string
	Excerpt            string
	Blocks             []publishingdomain.Block
	SEO                publishingdomain.SEOFields
	GEO                publishingdomain.GEOFields
	AEO                publishingdomain.AEOFields
	PublishedAt        *time.Time
	UpdatedAt          time.Time
}

type ContentRepository interface {
	List(ctx context.Context, query ContentListQuery) (ContentListPage, error)
	Get(ctx context.Context, accountID, contentID, localeID string) (ContentDetail, error)
	FindPublic(ctx context.Context, accountID, localeID, path string) (ContentDetail, error)
}

type CreateContentCommand struct {
	Item        publishingdomain.ContentItem
	Translation publishingdomain.ContentTranslation
}

type UpdateContentCommand struct {
	AccountID       string
	ContentID       string
	TranslationID   string
	LocaleID        string
	ExpectedVersion int64
	Translation     publishingdomain.ContentTranslation
}

type DeleteContentCommand struct {
	AccountID       string
	ContentID       string
	TranslationID   string
	LocaleID        string
	ExpectedVersion int64
	DeletedAt       time.Time
}

type ContentCommandRepository interface {
	Create(context.Context, CreateContentCommand) (ContentDetail, error)
	Update(context.Context, UpdateContentCommand) (ContentDetail, error)
	SoftDelete(context.Context, DeleteContentCommand) error
}

func ValidateContentListQuery(query ContentListQuery) error {
	if !contentIDPattern.MatchString(query.AccountID) ||
		(!contentIDPattern.MatchString(query.LocaleID) && !validLocaleTag(query.LocaleID)) ||
		query.Limit < 1 || query.Limit > 100 {
		return ErrInvalidContentQuery
	}
	if query.Kind != nil && *query.Kind != publishingdomain.KindPage &&
		*query.Kind != publishingdomain.KindPost {
		return ErrInvalidContentQuery
	}
	for _, status := range query.Status {
		if publishingdomain.ValidateStatus(status) != nil {
			return ErrInvalidContentQuery
		}
	}
	if query.After != nil &&
		(query.After.UpdatedAt.IsZero() || !contentIDPattern.MatchString(query.After.ID)) {
		return ErrInvalidContentQuery
	}
	return nil
}

func validLocaleTag(value string) bool {
	_, err := publishingdomain.NormalizeLocale(value)
	return err == nil
}
