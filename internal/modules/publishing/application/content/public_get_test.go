package content

import (
	"context"
	"testing"

	publishingdomain "github.com/korwlik23/go-api-starter/internal/modules/publishing/domain"
	"github.com/korwlik23/go-api-starter/internal/modules/publishing/ports"
	"github.com/stretchr/testify/require"
)

const (
	publicAccountID       = "018f1f64-7b2a-7c10-8bf2-3c40d5f67580"
	publicContentID       = "018f1f64-7b2a-7c10-8bf2-3c40d5f67581"
	publicTranslationID   = "018f1f64-7b2a-7c10-8bf2-3c40d5f67582"
	publicEnglishLocaleID = "018f1f64-7b2a-7c10-8bf2-3c40d5f67583"
	publicFrenchLocaleID  = "018f1f64-7b2a-7c10-8bf2-3c40d5f67584"
	publicGermanLocaleID  = "018f1f64-7b2a-7c10-8bf2-3c40d5f67585"
)

func TestM3C1PublicResolverReturnsOnlyPublishedAlternateTranslations(t *testing.T) {
	t.Parallel()

	repository := &publicContentRepositoryFake{
		detail: publicContentDetail(publicEnglishLocaleID, publishingdomain.StatusPublished),
		alternates: []ports.ContentDetail{
			publicContentDetail(publicFrenchLocaleID, publishingdomain.StatusPublished),
			publicContentDetail(publicGermanLocaleID, publishingdomain.StatusDraft),
			func() ports.ContentDetail {
				item := publicContentDetail(publicGermanLocaleID, publishingdomain.StatusPublished)
				item.AccountID = "018f1f64-7b2a-7c10-8bf2-3c40d5f67586"
				return item
			}(),
		},
	}
	resolver, err := NewPublicResolver(repository)
	require.NoError(t, err)

	view, err := resolver.Execute(
		context.Background(),
		publicAccountID,
		publicEnglishLocaleID,
		"/Docs/Home",
	)
	require.NoError(t, err)
	require.Equal(t, "/docs/home", repository.findPath)
	require.Equal(t, publishingdomain.StatusPublished, view.Content.ContentStatus)
	require.Equal(t, publishingdomain.StatusPublished, view.Content.TranslationStatus)
	require.Len(t, view.Alternates, 1)
	require.Equal(t, publicFrenchLocaleID, view.Alternates[0].LocaleID)
	require.NotEqual(t, publishingdomain.StatusDraft, view.Alternates[0].TranslationStatus)
}

func TestM3C1PublicResolverMapsMissingTranslationToStableNotFound(t *testing.T) {
	t.Parallel()

	repository := &publicContentRepositoryFake{detailErr: ports.ErrContentNotFound}
	resolver, err := NewPublicResolver(repository)
	require.NoError(t, err)

	_, err = resolver.Execute(
		context.Background(),
		publicAccountID,
		publicEnglishLocaleID,
		"/missing",
	)
	require.ErrorIs(t, err, ports.ErrContentNotFound)
}

func TestM3C1PublicResolverRejectsInvalidScopeBeforeRepositoryLookup(t *testing.T) {
	t.Parallel()

	repository := &publicContentRepositoryFake{}
	resolver, err := NewPublicResolver(repository)
	require.NoError(t, err)

	_, err = resolver.Execute(context.Background(), "", publicEnglishLocaleID, "/home")
	require.ErrorIs(t, err, ErrInvalidPublicContentQuery)
	require.Equal(t, 0, repository.findCalls)
}

type publicContentRepositoryFake struct {
	detail       ports.ContentDetail
	detailErr    error
	alternates   []ports.ContentDetail
	alternateErr error
	findCalls    int
	posts        ports.ContentListPage
	postsErr     error
	findPath     string
}

func (repository *publicContentRepositoryFake) FindPublic(
	_ context.Context,
	_ string,
	_ string,
	path string,
) (ports.ContentDetail, error) {
	repository.findCalls++
	repository.findPath = path
	if repository.detailErr != nil {
		return ports.ContentDetail{}, repository.detailErr
	}
	return repository.detail, nil
}

func (repository *publicContentRepositoryFake) ListPublishedAlternates(
	context.Context,
	string,
	string,
	string,
) ([]ports.ContentDetail, error) {
	if repository.alternateErr != nil {
		return nil, repository.alternateErr
	}
	return repository.alternates, nil
}

func (repository *publicContentRepositoryFake) ListPublishedPosts(
	context.Context,
	string,
	string,
	int,
	*ports.ContentCursor,
) (ports.ContentListPage, error) {
	return repository.posts, repository.postsErr
}

func publicContentDetail(localeID string, status publishingdomain.Status) ports.ContentDetail {
	return ports.ContentDetail{
		ContentID: publicContentID, TranslationID: publicTranslationID,
		AccountID: publicAccountID, LocaleID: localeID,
		Kind: publishingdomain.KindPage, ContentKey: "home",
		ContentStatus: status, ContentVersion: 2,
		TranslationStatus: status, TranslationVersion: 3,
		Slug: "home", Path: "/home", Title: "Home",
		Blocks: []publishingdomain.Block{{
			Type: publishingdomain.BlockText,
			Data: map[string]any{"text": "Welcome"},
		}},
	}
}

var _ PublicContentRepository = (*publicContentRepositoryFake)(nil)
