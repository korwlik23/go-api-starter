package content

import (
	"context"
	"errors"
	"sort"
	"strings"

	publishingdomain "github.com/korwlik23/go-api-starter/internal/modules/publishing/domain"
	"github.com/korwlik23/go-api-starter/internal/modules/publishing/ports"
)

var (
	ErrInvalidPublicResolver     = errors.New("invalid public content resolver")
	ErrInvalidPublicContentQuery = errors.New("invalid public content query")
)

// PublicContentRepository is deliberately separate from ContentRepository so
// public reads cannot accidentally widen the mutation/admin contract.
type PublicContentRepository interface {
	FindPublic(context.Context, string, string, string) (ports.ContentDetail, error)
	ListPublishedAlternates(context.Context, string, string, string) ([]ports.ContentDetail, error)
}

type PublicContentView struct {
	Content    ports.ContentDetail
	Alternates []ports.ContentDetail
}

type PublicResolver struct {
	repository PublicContentRepository
}

func NewPublicResolver(repository PublicContentRepository) (*PublicResolver, error) {
	if repository == nil {
		return nil, ErrInvalidPublicResolver
	}
	return &PublicResolver{repository: repository}, nil
}

// Execute returns a single published translation and only published
// alternates. The repository is expected to enforce those predicates in SQL;
// the application layer repeats them before serializing a public response so
// a faulty adapter cannot leak drafts or another account's content.
func (resolver *PublicResolver) Execute(
	ctx context.Context,
	accountID string,
	localeID string,
	path string,
) (PublicContentView, error) {
	if resolver == nil || resolver.repository == nil || ctx == nil {
		return PublicContentView{}, ErrInvalidPublicResolver
	}
	if err := validatePublicScope(accountID, localeID, path); err != nil {
		return PublicContentView{}, err
	}
	normalizedPath, _ := publishingdomain.NormalizePath(path)
	detail, err := resolver.repository.FindPublic(ctx, accountID, localeID, normalizedPath)
	if err != nil {
		return PublicContentView{}, err
	}
	if !isPublishedDetail(detail, accountID) || detail.LocaleID != localeID {
		return PublicContentView{}, ports.ErrContentNotFound
	}
	alternates, err := resolver.repository.ListPublishedAlternates(
		ctx, accountID, detail.ContentID, localeID,
	)
	if err != nil {
		return PublicContentView{}, err
	}
	visibleAlternates := make([]ports.ContentDetail, 0, len(alternates))
	seenLocales := make(map[string]struct{}, len(alternates))
	for _, alternate := range alternates {
		if !isPublishedDetail(alternate, accountID) {
			continue
		}
		if alternate.ContentID != detail.ContentID || alternate.LocaleID == localeID {
			continue
		}
		if _, seen := seenLocales[alternate.LocaleID]; seen {
			continue
		}
		seenLocales[alternate.LocaleID] = struct{}{}
		visibleAlternates = append(visibleAlternates, alternate)
	}
	sort.SliceStable(visibleAlternates, func(left, right int) bool {
		if visibleAlternates[left].LocaleID == visibleAlternates[right].LocaleID {
			return visibleAlternates[left].Path < visibleAlternates[right].Path
		}
		return visibleAlternates[left].LocaleID < visibleAlternates[right].LocaleID
	})
	return PublicContentView{Content: detail, Alternates: visibleAlternates}, nil
}

func validatePublicScope(accountID, localeID, path string) error {
	query := ports.ContentListQuery{AccountID: accountID, LocaleID: localeID, Limit: 1}
	if err := ports.ValidateContentListQuery(query); err != nil {
		return ErrInvalidPublicContentQuery
	}
	if _, err := publishingdomain.NormalizePath(path); err != nil {
		return ErrInvalidPublicContentQuery
	}
	return nil
}

func isPublishedDetail(detail ports.ContentDetail, accountID string) bool {
	if detail.AccountID != accountID || detail.ContentID == "" ||
		detail.TranslationID == "" || detail.LocaleID == "" ||
		detail.ContentVersion < 1 ||
		detail.TranslationVersion < 1 || detail.ContentStatus != publishingdomain.StatusPublished ||
		detail.TranslationStatus != publishingdomain.StatusPublished ||
		(detail.Kind != publishingdomain.KindPage && detail.Kind != publishingdomain.KindPost) ||
		strings.TrimSpace(detail.Title) == "" {
		return false
	}
	_, err := publishingdomain.NormalizePath(detail.Path)
	return err == nil
}
