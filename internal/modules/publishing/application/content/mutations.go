package content

import (
	"context"
	"errors"
	"time"

	publishingdomain "github.com/korwlik23/go-api-starter/internal/modules/publishing/domain"
	"github.com/korwlik23/go-api-starter/internal/modules/publishing/ports"
	"github.com/korwlik23/go-api-starter/internal/platform/database"
)

var ErrInvalidContentMutation = errors.New("publishing: invalid content mutation")

type ContentAuditEvent struct {
	ActorUserID   string
	AccountID     string
	RequestID     string
	Action        string
	ContentID     string
	TranslationID string
	Version       int64
	Data          map[string]any
}

type ContentAuditor interface {
	RecordContentChange(context.Context, ContentAuditEvent) error
}

type ContentMutationDependencies struct {
	Repository   ports.ContentCommandRepository
	Transactions database.Transactor
	Auditor      ContentAuditor
	NewID        func() (string, error)
	Now          func() time.Time
}

type ContentMutations struct {
	dependencies ContentMutationDependencies
}

func NewContentMutations(
	dependencies ContentMutationDependencies,
) (*ContentMutations, error) {
	if dependencies.Repository == nil ||
		dependencies.Auditor == nil ||
		dependencies.NewID == nil ||
		dependencies.Now == nil {
		return nil, ErrInvalidContentMutation
	}
	return &ContentMutations{dependencies: dependencies}, nil
}

type CreateInput struct {
	AccountID   string
	ActorUserID string
	RequestID   string
	Kind        publishingdomain.Kind
	ContentKey  string
	Locale      string
	Slug        string
	Path        string
	Title       string
	Excerpt     string
	Blocks      []publishingdomain.Block
	SEO         publishingdomain.SEOFields
	GEO         publishingdomain.GEOFields
	AEO         publishingdomain.AEOFields
}

type UpdateInput struct {
	AccountID       string
	ActorUserID     string
	RequestID       string
	ContentID       string
	TranslationID   string
	ExpectedVersion int64
	Locale          string
	Slug            string
	Path            string
	Title           string
	Excerpt         string
	Blocks          []publishingdomain.Block
	SEO             publishingdomain.SEOFields
	GEO             publishingdomain.GEOFields
	AEO             publishingdomain.AEOFields
}

type DeleteInput struct {
	AccountID       string
	ActorUserID     string
	RequestID       string
	ContentID       string
	TranslationID   string
	LocaleID        string
	ExpectedVersion int64
}

func (service *ContentMutations) runMutation(
	ctx context.Context,
	operation func(context.Context) error,
) error {
	if service.dependencies.Transactions == nil {
		return operation(ctx)
	}
	return service.dependencies.Transactions.WithinTransaction(ctx, operation)
}
