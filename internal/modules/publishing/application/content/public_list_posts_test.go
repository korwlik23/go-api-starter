package content

import (
	"context"
	"testing"

	publishingdomain "github.com/korwlik23/go-api-starter/internal/modules/publishing/domain"
	"github.com/korwlik23/go-api-starter/internal/modules/publishing/ports"
	"github.com/stretchr/testify/require"
)

func TestM3C1PublicPostsListExcludesUnpublishedItemsAndCrossAccountRows(t *testing.T) {
	t.Parallel()

	repository := &publicContentRepositoryFake{posts: ports.ContentListPage{Items: []ports.ContentListItem{
		{
			ContentID: publicContentID, AccountID: publicAccountID,
			Kind: publishingdomain.KindPost, ContentStatus: publishingdomain.StatusPublished,
			ContentVersion: 2, TranslationID: publicTranslationID,
			LocaleID: publicEnglishLocaleID, TranslationStatus: publishingdomain.StatusPublished,
			TranslationVersion: 3, Path: "/news/hello", Title: "Hello",
		},
		{
			ContentID: "018f1f64-7b2a-7c10-8bf2-3c40d5f67587", AccountID: publicAccountID,
			Kind: publishingdomain.KindPost, ContentStatus: publishingdomain.StatusPublished,
			ContentVersion: 2, TranslationID: "018f1f64-7b2a-7c10-8bf2-3c40d5f67588",
			LocaleID: publicEnglishLocaleID, TranslationStatus: publishingdomain.StatusDraft,
			TranslationVersion: 3, Path: "/news/draft", Title: "Draft",
		},
		{
			ContentID: "018f1f64-7b2a-7c10-8bf2-3c40d5f67589", AccountID: "018f1f64-7b2a-7c10-8bf2-3c40d5f67586",
			Kind: publishingdomain.KindPost, ContentStatus: publishingdomain.StatusPublished,
			ContentVersion: 2, TranslationID: "018f1f64-7b2a-7c10-8bf2-3c40d5f67590",
			LocaleID: publicEnglishLocaleID, TranslationStatus: publishingdomain.StatusPublished,
			TranslationVersion: 3, Path: "/news/other", Title: "Other",
		},
	}}}
	service, err := NewPublicPosts(repository)
	require.NoError(t, err)

	page, err := service.Execute(context.Background(), PublicPostsInput{
		AccountID: publicAccountID, LocaleID: publicEnglishLocaleID, Limit: 20,
	})
	require.NoError(t, err)
	require.Len(t, page.Items, 1)
	require.Equal(t, "/news/hello", page.Items[0].Path)
	require.Equal(t, publishingdomain.StatusPublished, page.Items[0].TranslationStatus)
}

func TestM3C1PublicPostsListRequiresBoundedPublishedQuery(t *testing.T) {
	t.Parallel()

	service, err := NewPublicPosts(&publicContentRepositoryFake{})
	require.NoError(t, err)

	_, err = service.Execute(context.Background(), PublicPostsInput{
		AccountID: publicAccountID, LocaleID: publicEnglishLocaleID, Limit: 101,
	})
	require.ErrorIs(t, err, ErrInvalidPublicContentQuery)
}
