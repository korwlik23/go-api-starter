package content

import (
	"context"
	"errors"
	"strings"

	publishingdomain "github.com/korwlik23/go-api-starter/internal/modules/publishing/domain"
	"github.com/korwlik23/go-api-starter/internal/modules/publishing/ports"
)

var ErrInvalidContentQuery = errors.New("publishing: invalid content query")

type ContentQueryDependencies struct {
	Repository ports.ContentRepository
}

type ContentQueries struct {
	repository ports.ContentRepository
}

func NewContentQueries(dependencies ContentQueryDependencies) (*ContentQueries, error) {
	if dependencies.Repository == nil {
		return nil, ErrInvalidContentQuery
	}
	return &ContentQueries{repository: dependencies.Repository}, nil
}

type ListInput struct {
	AccountID string
	Locale    string
	Kind      *publishingdomain.Kind
	Status    []publishingdomain.Status
	After     *ports.ContentCursor
	Limit     int
}

func (queries *ContentQueries) List(ctx context.Context, input ListInput) (ports.ContentListPage, error) {
	if queries == nil || queries.repository == nil || ctx == nil ||
		strings.TrimSpace(input.AccountID) == "" || strings.TrimSpace(input.Locale) == "" {
		return ports.ContentListPage{}, ErrInvalidContentQuery
	}
	locale, err := publishingdomain.NormalizeLocale(input.Locale)
	if err != nil {
		return ports.ContentListPage{}, ErrInvalidContentQuery
	}
	return queries.repository.List(ctx, ports.ContentListQuery{
		AccountID: input.AccountID, LocaleID: locale, Kind: input.Kind,
		Status: input.Status, After: input.After, Limit: input.Limit,
	})
}

type GetInput struct {
	AccountID string
	ContentID string
	Locale    string
}

func (queries *ContentQueries) Get(ctx context.Context, input GetInput) (ports.ContentDetail, error) {
	if queries == nil || queries.repository == nil || ctx == nil ||
		strings.TrimSpace(input.AccountID) == "" || strings.TrimSpace(input.ContentID) == "" ||
		strings.TrimSpace(input.Locale) == "" {
		return ports.ContentDetail{}, ErrInvalidContentQuery
	}
	locale, err := publishingdomain.NormalizeLocale(input.Locale)
	if err != nil {
		return ports.ContentDetail{}, ErrInvalidContentQuery
	}
	return queries.repository.Get(ctx, input.AccountID, input.ContentID, locale)
}
