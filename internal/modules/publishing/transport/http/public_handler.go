package http

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	localizationdomain "github.com/korwlik23/go-api-starter/internal/modules/localization/domain"
	navigationdomain "github.com/korwlik23/go-api-starter/internal/modules/navigation/domain"
	navigationports "github.com/korwlik23/go-api-starter/internal/modules/navigation/ports"
	contentapp "github.com/korwlik23/go-api-starter/internal/modules/publishing/application/content"
	publishingdomain "github.com/korwlik23/go-api-starter/internal/modules/publishing/domain"
	"github.com/korwlik23/go-api-starter/internal/modules/publishing/ports"
	platformmiddleware "github.com/korwlik23/go-api-starter/internal/platform/middleware"
	"github.com/korwlik23/go-api-starter/internal/platform/response"
)

var ErrInvalidPublicHandler = errors.New("invalid public publishing handler dependencies")

type publicContentReader interface {
	Execute(context.Context, string, string, string) (contentapp.PublicContentView, error)
}

type publicPostsReader interface {
	Execute(context.Context, contentapp.PublicPostsInput) (ports.ContentListPage, error)
}

type publicLocaleReader interface {
	FindByTag(context.Context, string) (localizationdomain.Locale, error)
}

type publicMenuReader interface {
	FindPublic(context.Context, string, string, string) (navigationdomain.Menu, error)
}

type PublicHandler struct {
	content   publicContentReader
	posts     publicPostsReader
	locales   publicLocaleReader
	menus     publicMenuReader
	accountID string
}

func NewPublicHandler(
	content publicContentReader,
	posts publicPostsReader,
	locales publicLocaleReader,
	accountID string,
) (*PublicHandler, error) {
	return NewPublicHandlerWithMenus(content, posts, locales, nil, accountID)
}

func NewPublicHandlerWithMenus(
	content publicContentReader,
	posts publicPostsReader,
	locales publicLocaleReader,
	menus publicMenuReader,
	accountID string,
) (*PublicHandler, error) {
	if content == nil || posts == nil || locales == nil || strings.TrimSpace(accountID) == "" {
		return nil, ErrInvalidPublicHandler
	}
	return &PublicHandler{
		content: content, posts: posts, locales: locales, menus: menus,
		accountID: strings.TrimSpace(accountID),
	}, nil
}

func (handler *PublicHandler) GetContent(ctx *gin.Context) {
	handler.getContent(ctx, nil)
}

func (handler *PublicHandler) GetPost(ctx *gin.Context) {
	kind := publishingdomain.KindPost
	handler.getContent(ctx, &kind)
}

func (handler *PublicHandler) getContent(ctx *gin.Context, expectedKind *publishingdomain.Kind) {
	locale, ok := handler.resolveLocale(ctx)
	if !ok {
		return
	}
	view, err := handler.content.Execute(
		ctx.Request.Context(), handler.accountID, locale.ID, publicPathParam(ctx),
	)
	if err != nil {
		writePublicError(ctx, err)
		return
	}
	if expectedKind != nil && view.Content.Kind != *expectedKind {
		writePublicNotFound(ctx)
		return
	}
	ctx.Header("Cache-Control", "public, max-age=60")
	ctx.JSON(http.StatusOK, publicContentResponseFromView(view))
}

func publicPathParam(ctx *gin.Context) string {
	if path := ctx.Param("path"); path != "" {
		return path
	}
	if slug := strings.TrimSpace(ctx.Param("slug")); slug != "" {
		return "/" + slug
	}
	return ""
}

func (handler *PublicHandler) GetMenu(ctx *gin.Context) {
	locale, ok := handler.resolveLocale(ctx)
	if !ok {
		return
	}
	if handler.menus == nil {
		writePublicNotFound(ctx)
		return
	}
	menu, err := handler.menus.FindPublic(
		ctx.Request.Context(), handler.accountID, locale.ID, ctx.Param("location"),
	)
	if err != nil {
		if errors.Is(err, navigationports.ErrMenuNotFound) {
			writePublicMenuNotFound(ctx)
			return
		}
		writePublicError(ctx, err)
		return
	}
	ctx.Header("Cache-Control", "public, max-age=60")
	ctx.JSON(http.StatusOK, publicMenuResponseFromDomain(menu))
}

func (handler *PublicHandler) ListPosts(ctx *gin.Context) {
	locale, ok := handler.resolveLocale(ctx)
	if !ok {
		return
	}
	limit := 20
	if raw := strings.TrimSpace(ctx.Query("limit")); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 1 || parsed > 100 {
			writePublicProblem(ctx, http.StatusBadRequest, "invalid_request", "The public content request is invalid.")
			return
		}
		limit = parsed
	}
	page, err := handler.posts.Execute(ctx.Request.Context(), contentapp.PublicPostsInput{
		AccountID: handler.accountID, LocaleID: locale.ID, Limit: limit,
	})
	if err != nil {
		writePublicError(ctx, err)
		return
	}
	items := make([]publicPostItem, 0, len(page.Items))
	for _, item := range page.Items {
		items = append(items, publicPostItem{
			ContentID: item.ContentID, TranslationID: item.TranslationID,
			LocaleID: item.LocaleID, Path: item.Path, Slug: item.Slug,
			Title: item.Title, UpdatedAt: item.UpdatedAt,
		})
	}
	responseBody := publicPostListResponse{Items: items}
	if page.Next != nil {
		responseBody.Next = &publicPostCursor{
			UpdatedAt: page.Next.UpdatedAt, ID: page.Next.ID,
		}
	}
	ctx.Header("Cache-Control", "public, max-age=60")
	ctx.JSON(http.StatusOK, responseBody)
}

func (handler *PublicHandler) resolveLocale(ctx *gin.Context) (localizationdomain.Locale, bool) {
	tag, err := localizationdomain.NormalizeLocaleTag(ctx.Param("locale"))
	if err != nil {
		writePublicNotFound(ctx)
		return localizationdomain.Locale{}, false
	}
	locale, err := handler.locales.FindByTag(ctx.Request.Context(), tag)
	if err != nil || !locale.Enabled || strings.TrimSpace(locale.ID) == "" || locale.Tag != tag {
		writePublicNotFound(ctx)
		return localizationdomain.Locale{}, false
	}
	return locale, true
}

type publicContentResponse struct {
	ContentID          string                   `json:"content_id"`
	TranslationID      string                   `json:"translation_id"`
	LocaleID           string                   `json:"locale_id"`
	Kind               publishingdomain.Kind    `json:"kind"`
	ContentKey         string                   `json:"content_key"`
	ContentStatus      publishingdomain.Status  `json:"content_status"`
	ContentVersion     int64                    `json:"content_version"`
	TranslationStatus  publishingdomain.Status  `json:"translation_status"`
	TranslationVersion int64                    `json:"translation_version"`
	PublishedAt        *time.Time               `json:"published_at,omitempty"`
	UpdatedAt          time.Time                `json:"updated_at"`
	Slug               string                   `json:"slug"`
	Path               string                   `json:"path"`
	Title              string                   `json:"title"`
	Excerpt            string                   `json:"excerpt"`
	Blocks             []publishingdomain.Block `json:"blocks"`
	SEO                publicSEOFields          `json:"seo"`
	GEO                publicGEOFields          `json:"geo"`
	AEO                publicAEOFields          `json:"aeo"`
	Alternates         []publicAlternate        `json:"alternates"`
}

type publicAlternate struct {
	TranslationID string `json:"translation_id"`
	LocaleID      string `json:"locale_id"`
	Slug          string `json:"slug"`
	Path          string `json:"path"`
	Title         string `json:"title"`
}

type publicSEOFields struct {
	Title          string         `json:"title"`
	Description    string         `json:"description"`
	CanonicalURL   string         `json:"canonical_url,omitempty"`
	Robots         string         `json:"robots,omitempty"`
	StructuredData map[string]any `json:"structured_data,omitempty"`
}

type publicGEOFields struct {
	Region   string `json:"region,omitempty"`
	Locality string `json:"locality,omitempty"`
}

type publicAEOFields struct {
	Question string `json:"question,omitempty"`
	Answer   string `json:"answer,omitempty"`
}

type publicPostListResponse struct {
	Items []publicPostItem  `json:"items"`
	Next  *publicPostCursor `json:"next,omitempty"`
}

type publicPostItem struct {
	ContentID     string    `json:"content_id"`
	TranslationID string    `json:"translation_id"`
	LocaleID      string    `json:"locale_id"`
	Slug          string    `json:"slug"`
	Path          string    `json:"path"`
	Title         string    `json:"title"`
	UpdatedAt     time.Time `json:"updated_at"`
}

type publicPostCursor struct {
	UpdatedAt time.Time `json:"updated_at"`
	ID        string    `json:"id"`
}

type publicMenuResponse struct {
	ID       string           `json:"id"`
	LocaleID string           `json:"locale_id"`
	Key      string           `json:"key"`
	Name     string           `json:"name"`
	Version  int64            `json:"version"`
	Items    []publicMenuItem `json:"items"`
}

type publicMenuItem struct {
	ID         string                      `json:"id"`
	ParentID   string                      `json:"parent_id,omitempty"`
	Label      string                      `json:"label"`
	TargetKind navigationdomain.TargetKind `json:"target_kind"`
	Target     string                      `json:"target"`
	Position   int64                       `json:"position"`
	Children   []publicMenuItem            `json:"children,omitempty"`
}

func publicContentResponseFromView(view contentapp.PublicContentView) publicContentResponse {
	detail := view.Content
	result := publicContentResponse{
		ContentID: detail.ContentID, TranslationID: detail.TranslationID,
		LocaleID: detail.LocaleID, Kind: detail.Kind, ContentKey: detail.ContentKey,
		ContentStatus: detail.ContentStatus, ContentVersion: detail.ContentVersion,
		TranslationStatus: detail.TranslationStatus, TranslationVersion: detail.TranslationVersion,
		PublishedAt: detail.PublishedAt, UpdatedAt: detail.UpdatedAt,
		Slug: detail.Slug, Path: detail.Path, Title: detail.Title, Excerpt: detail.Excerpt,
		Blocks: detail.Blocks,
		SEO: publicSEOFields{
			Title: detail.SEO.Title, Description: detail.SEO.Description,
			CanonicalURL: detail.SEO.CanonicalURL, Robots: detail.SEO.Robots,
			StructuredData: detail.SEO.StructuredData,
		},
		GEO:        publicGEOFields{Region: detail.GEO.Region, Locality: detail.GEO.Locality},
		AEO:        publicAEOFields{Question: detail.AEO.Question, Answer: detail.AEO.Answer},
		Alternates: make([]publicAlternate, 0, len(view.Alternates)),
	}
	for _, alternate := range view.Alternates {
		result.Alternates = append(result.Alternates, publicAlternate{
			TranslationID: alternate.TranslationID, LocaleID: alternate.LocaleID,
			Slug: alternate.Slug, Path: alternate.Path, Title: alternate.Title,
		})
	}
	return result
}

func publicMenuResponseFromDomain(menu navigationdomain.Menu) publicMenuResponse {
	return publicMenuResponse{
		ID: menu.ID, LocaleID: menu.LocaleID, Key: menu.Key, Name: menu.Name,
		Version: menu.Version, Items: publicMenuItems(menu.Items),
	}
}

func publicMenuItems(items []navigationdomain.MenuItem) []publicMenuItem {
	result := make([]publicMenuItem, 0, len(items))
	for _, item := range items {
		result = append(result, publicMenuItem{
			ID: item.ID, ParentID: item.ParentID, Label: item.Label,
			TargetKind: item.TargetKind, Target: item.Target, Position: item.Position,
			Children: publicMenuItems(item.Children),
		})
	}
	return result
}

func writePublicError(ctx *gin.Context, err error) {
	switch {
	case errors.Is(err, ports.ErrContentNotFound), errors.Is(err, contentapp.ErrInvalidPublicContentQuery):
		writePublicNotFound(ctx)
	case errors.Is(err, ports.ErrInvalidContentQuery):
		writePublicProblem(ctx, http.StatusBadRequest, "invalid_request", "The public content request is invalid.")
	default:
		writePublicProblem(ctx, http.StatusInternalServerError, "internal_error", "The public content request could not be completed.")
	}
}

func writePublicNotFound(ctx *gin.Context) {
	writePublicProblem(ctx, http.StatusNotFound, "public_content_not_found", "The requested content was not found.")
}

func writePublicMenuNotFound(ctx *gin.Context) {
	writePublicProblem(ctx, http.StatusNotFound, "public_menu_not_found", "The requested public menu was not found.")
}

func writePublicProblem(ctx *gin.Context, status int, code, message string) {
	response.WriteProblem(
		ctx, status,
		platformmiddleware.RequestIDFromContext(ctx.Request.Context()),
		response.NewProblem(code, message, nil, nil),
	)
}
