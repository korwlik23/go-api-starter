package http

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	localizationdomain "github.com/korwlik23/go-api-starter/internal/modules/localization/domain"
	contentapp "github.com/korwlik23/go-api-starter/internal/modules/publishing/application/content"
	publishingdomain "github.com/korwlik23/go-api-starter/internal/modules/publishing/domain"
	"github.com/korwlik23/go-api-starter/internal/modules/publishing/ports"
	"github.com/stretchr/testify/require"
)

func TestM3C1PublicHandlerRejectsMissingDependencies(t *testing.T) {
	t.Parallel()

	handler, err := NewPublicHandler(nil, nil, nil, "")
	require.Nil(t, handler)
	require.ErrorIs(t, err, ErrInvalidPublicHandler)
}

func TestM3C1PublicHandlerReturnsPublishedContentAndStable404(t *testing.T) {
	t.Parallel()

	resolver := &publicContentReaderFake{
		view: contentapp.PublicContentView{
			Content: publicHTTPDetail(publishingdomain.StatusPublished),
			Alternates: []ports.ContentDetail{
				publicHTTPAlternate("018f1f64-7b2a-7c10-8bf2-3c40d5f67584"),
			},
		},
	}
	locales := &publicLocaleReaderFake{locale: localizationdomain.Locale{
		ID: "018f1f64-7b2a-7c10-8bf2-3c40d5f67583", Tag: "en", Enabled: true,
	}}
	handler, err := NewPublicHandler(
		resolver,
		&publicPostsReaderFake{},
		locales,
		"018f1f64-7b2a-7c10-8bf2-3c40d5f67580",
	)
	require.NoError(t, err)

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/public/content/:locale/*path", handler.GetContent)

	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/public/content/en/docs/home", nil)
	router.ServeHTTP(response, request)
	require.Equal(t, http.StatusOK, response.Code)
	require.Equal(t, "public, max-age=60", response.Header().Get("Cache-Control"))
	var body map[string]any
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &body))
	require.Equal(t, "published", body["content_status"])
	require.NotContains(t, response.Body.String(), "draft")
	require.Len(t, body["alternates"], 1)

	resolver.err = ports.ErrContentNotFound
	response = httptest.NewRecorder()
	request = httptest.NewRequest(http.MethodGet, "/public/content/en/docs/home", nil)
	router.ServeHTTP(response, request)
	require.Equal(t, http.StatusNotFound, response.Code)
	require.Contains(t, response.Body.String(), `"code":"public_content_not_found"`)
	require.NotContains(t, response.Body.String(), "content_id")
}

func TestM3C1PublicHandlerDoesNotRevealUnknownLocale(t *testing.T) {
	t.Parallel()

	locales := &publicLocaleReaderFake{err: ports.ErrContentNotFound}
	handler, err := NewPublicHandler(
		&publicContentReaderFake{}, &publicPostsReaderFake{}, locales,
		"018f1f64-7b2a-7c10-8bf2-3c40d5f67580",
	)
	require.NoError(t, err)

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/public/content/:locale/*path", handler.GetContent)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/public/content/zz/home", nil))
	require.Equal(t, http.StatusNotFound, response.Code)
	require.Contains(t, response.Body.String(), `"code":"public_content_not_found"`)
	require.NotContains(t, response.Body.String(), "locale")
}

type publicContentReaderFake struct {
	view contentapp.PublicContentView
	err  error
}

func (reader *publicContentReaderFake) Execute(
	context.Context,
	string,
	string,
	string,
) (contentapp.PublicContentView, error) {
	if reader.err != nil {
		return contentapp.PublicContentView{}, reader.err
	}
	return reader.view, nil
}

type publicPostsReaderFake struct {
	page ports.ContentListPage
	err  error
}

func (reader *publicPostsReaderFake) Execute(
	context.Context,
	contentapp.PublicPostsInput,
) (ports.ContentListPage, error) {
	return reader.page, reader.err
}

type publicLocaleReaderFake struct {
	locale localizationdomain.Locale
	err    error
}

func (reader *publicLocaleReaderFake) FindByTag(context.Context, string) (localizationdomain.Locale, error) {
	if reader.err != nil {
		return localizationdomain.Locale{}, reader.err
	}
	return reader.locale, nil
}

func publicHTTPDetail(status publishingdomain.Status) ports.ContentDetail {
	return ports.ContentDetail{
		ContentID:     "018f1f64-7b2a-7c10-8bf2-3c40d5f67581",
		TranslationID: "018f1f64-7b2a-7c10-8bf2-3c40d5f67582",
		AccountID:     "018f1f64-7b2a-7c10-8bf2-3c40d5f67580",
		LocaleID:      "018f1f64-7b2a-7c10-8bf2-3c40d5f67583",
		Kind:          publishingdomain.KindPage, ContentKey: "home",
		ContentStatus: status, ContentVersion: 2,
		TranslationStatus: status, TranslationVersion: 3,
		Slug: "home", Path: "/docs/home", Title: "Home", Excerpt: "Welcome",
		Blocks: []publishingdomain.Block{{Type: publishingdomain.BlockText, Data: map[string]any{"text": "Welcome"}}},
		SEO:    publishingdomain.SEOFields{Title: "Home", Description: "Welcome"},
	}
}

func publicHTTPAlternate(localeID string) ports.ContentDetail {
	detail := publicHTTPDetail(publishingdomain.StatusPublished)
	detail.LocaleID = localeID
	return detail
}
