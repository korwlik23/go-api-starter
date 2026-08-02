package content

import (
	"context"
	"strings"

	publishingdomain "github.com/korwlik23/go-api-starter/internal/modules/publishing/domain"
	"github.com/korwlik23/go-api-starter/internal/modules/publishing/ports"
)

type PublicPostsRepository interface {
	ListPublishedPosts(context.Context, string, string, int, *ports.ContentCursor) (ports.ContentListPage, error)
}

type PublicPostsInput struct {
	AccountID string
	LocaleID  string
	Limit     int
	After     *ports.ContentCursor
}

type PublicPosts struct {
	repository PublicPostsRepository
}

func NewPublicPosts(repository PublicPostsRepository) (*PublicPosts, error) {
	if repository == nil {
		return nil, ErrInvalidPublicResolver
	}
	return &PublicPosts{repository: repository}, nil
}

func (service *PublicPosts) Execute(
	ctx context.Context,
	input PublicPostsInput,
) (ports.ContentListPage, error) {
	if service == nil || service.repository == nil || ctx == nil {
		return ports.ContentListPage{}, ErrInvalidPublicResolver
	}
	kind := publishingdomain.KindPost
	if err := ports.ValidateContentListQuery(ports.ContentListQuery{
		AccountID: input.AccountID, LocaleID: input.LocaleID, Kind: &kind,
		Limit: input.Limit, After: input.After,
	}); err != nil {
		return ports.ContentListPage{}, ErrInvalidPublicContentQuery
	}
	page, err := service.repository.ListPublishedPosts(
		ctx, input.AccountID, input.LocaleID, input.Limit, input.After,
	)
	if err != nil {
		return ports.ContentListPage{}, err
	}
	visible := make([]ports.ContentListItem, 0, len(page.Items))
	for _, item := range page.Items {
		if item.AccountID != input.AccountID || item.LocaleID != input.LocaleID ||
			item.Kind != publishingdomain.KindPost ||
			item.ContentStatus != publishingdomain.StatusPublished ||
			item.TranslationStatus != publishingdomain.StatusPublished ||
			item.ContentID == "" || item.TranslationID == "" ||
			item.ContentVersion < 1 || item.TranslationVersion < 1 ||
			strings.TrimSpace(item.Title) == "" {
			continue
		}
		if _, normalizeErr := publishingdomain.NormalizePath(item.Path); normalizeErr != nil {
			continue
		}
		visible = append(visible, item)
	}
	page.Items = visible
	return page, nil
}
