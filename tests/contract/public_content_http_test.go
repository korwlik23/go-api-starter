package contract_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	localizationdomain "github.com/korwlik23/go-api-starter/internal/modules/localization/domain"
	navigationdomain "github.com/korwlik23/go-api-starter/internal/modules/navigation/domain"
	contentapp "github.com/korwlik23/go-api-starter/internal/modules/publishing/application/content"
	publishingdomain "github.com/korwlik23/go-api-starter/internal/modules/publishing/domain"
	"github.com/korwlik23/go-api-starter/internal/modules/publishing/ports"
	publishinghttp "github.com/korwlik23/go-api-starter/internal/modules/publishing/transport/http"
	"github.com/stretchr/testify/require"
)

func TestM3C1PublicContentHTTPDoesNotExposeDraftLocales(t *testing.T) {
	t.Parallel()

	const (
		accountID = "018f1f64-7b2a-7c10-8bf2-3c40d5f67580"
		contentID = "018f1f64-7b2a-7c10-8bf2-3c40d5f67581"
		enID      = "018f1f64-7b2a-7c10-8bf2-3c40d5f67583"
		frID      = "018f1f64-7b2a-7c10-8bf2-3c40d5f67584"
	)
	repository := &publicContractRepositoryFake{
		detail: publicContractDetail(accountID, contentID, enID, publishingdomain.StatusPublished),
		alternates: []ports.ContentDetail{
			publicContractDetail(accountID, contentID, frID, publishingdomain.StatusPublished),
			publicContractDetail(accountID, contentID, "018f1f64-7b2a-7c10-8bf2-3c40d5f67585", publishingdomain.StatusDraft),
		},
		posts: ports.ContentListPage{Items: []ports.ContentListItem{
			{ContentID: contentID, AccountID: accountID, Kind: publishingdomain.KindPost,
				ContentStatus: publishingdomain.StatusPublished, ContentVersion: 2,
				TranslationID: "018f1f64-7b2a-7c10-8bf2-3c40d5f67582", LocaleID: enID,
				Slug: "hello", Path: "/news/hello", Title: "Hello",
				TranslationStatus: publishingdomain.StatusPublished, TranslationVersion: 3},
			{ContentID: "018f1f64-7b2a-7c10-8bf2-3c40d5f67586", AccountID: accountID, Kind: publishingdomain.KindPost,
				ContentStatus: publishingdomain.StatusPublished, ContentVersion: 2,
				TranslationID: "018f1f64-7b2a-7c10-8bf2-3c40d5f67587", LocaleID: enID,
				Slug: "draft", Path: "/news/draft", Title: "Draft",
				TranslationStatus: publishingdomain.StatusDraft, TranslationVersion: 3},
		}},
	}
	resolver, err := contentapp.NewPublicResolver(repository)
	require.NoError(t, err)
	posts, err := contentapp.NewPublicPosts(repository)
	require.NoError(t, err)
	handler, err := publishinghttp.NewPublicHandlerWithMenus(
		resolver, posts, publicContractLocaleReader{}, publicContractMenuReader{}, accountID,
	)
	require.NoError(t, err)

	gin.SetMode(gin.TestMode)
	router := gin.New()
	publishinghttp.RegisterPublicRoutes(router.Group("/api/v1"), handler)

	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/public/content/en/docs/home", nil))
	require.Equal(t, http.StatusOK, response.Code)
	require.NotContains(t, response.Body.String(), "draft")
	var contentBody map[string]any
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &contentBody))
	require.Equal(t, "published", contentBody["translation_status"])
	require.Len(t, contentBody["alternates"], 1)

	response = httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/public/posts/en?limit=20", nil))
	require.Equal(t, http.StatusOK, response.Code)
	require.NotContains(t, response.Body.String(), "draft")

	response = httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/public/posts/en/hello", nil))
	require.Equal(t, http.StatusOK, response.Code)
	require.Equal(t, "post", func() string {
		var postBody map[string]any
		require.NoError(t, json.Unmarshal(response.Body.Bytes(), &postBody))
		return postBody["kind"].(string)
	}())

	response = httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/public/menus/en/header", nil))
	require.Equal(t, http.StatusOK, response.Code)
	require.Contains(t, response.Body.String(), "Home")

	response = httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/public/content/en/missing", nil))
	require.Equal(t, http.StatusNotFound, response.Code)
}

type publicContractRepositoryFake struct {
	detail     ports.ContentDetail
	alternates []ports.ContentDetail
	posts      ports.ContentListPage
}

func (repository *publicContractRepositoryFake) FindPublic(_ context.Context, _ string, _ string, path string) (ports.ContentDetail, error) {
	if path == "/hello" {
		detail := repository.detail
		detail.Kind = publishingdomain.KindPost
		detail.Path = "/hello"
		return detail, nil
	}
	if path != "/docs/home" {
		return ports.ContentDetail{}, ports.ErrContentNotFound
	}
	return repository.detail, nil
}

func (repository *publicContractRepositoryFake) ListPublishedAlternates(context.Context, string, string, string) ([]ports.ContentDetail, error) {
	return repository.alternates, nil
}

func (repository *publicContractRepositoryFake) ListPublishedPosts(context.Context, string, string, int, *ports.ContentCursor) (ports.ContentListPage, error) {
	return repository.posts, nil
}

type publicContractLocaleReader struct{}

func (publicContractLocaleReader) FindByTag(_ context.Context, tag string) (localizationdomain.Locale, error) {
	if tag != "en" {
		return localizationdomain.Locale{}, ports.ErrContentNotFound
	}
	return localizationdomain.Locale{
		ID: "018f1f64-7b2a-7c10-8bf2-3c40d5f67583", Tag: "en", Enabled: true,
	}, nil
}

type publicContractMenuReader struct{}

func (publicContractMenuReader) FindPublic(context.Context, string, string, string) (navigationdomain.Menu, error) {
	return navigationdomain.Menu{
		ID:       "018f1f64-7b2a-7c10-8bf2-3c40d5f67590",
		LocaleID: "018f1f64-7b2a-7c10-8bf2-3c40d5f67583",
		Key:      "header", Name: "Header", Version: 1,
		Items: []navigationdomain.MenuItem{{
			ID:    "018f1f64-7b2a-7c10-8bf2-3c40d5f67591",
			Label: "Home", TargetKind: navigationdomain.TargetInternalPath,
			Target: "/", Position: 0, Version: 1,
		}},
	}, nil
}

func publicContractDetail(accountID, contentID, localeID string, status publishingdomain.Status) ports.ContentDetail {
	return ports.ContentDetail{
		ContentID: contentID, TranslationID: "018f1f64-7b2a-7c10-8bf2-3c40d5f67582",
		AccountID: accountID, LocaleID: localeID, Kind: publishingdomain.KindPage,
		ContentKey: "home", ContentStatus: status, ContentVersion: 2,
		TranslationStatus: status, TranslationVersion: 3, Slug: "home",
		Path: "/docs/home", Title: "Home",
		Blocks: []publishingdomain.Block{{Type: publishingdomain.BlockText, Data: map[string]any{"text": "Welcome"}}},
	}
}
