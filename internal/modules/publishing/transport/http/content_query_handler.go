package http

import (
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	contentapp "github.com/korwlik23/go-api-starter/internal/modules/publishing/application/content"
	publishingdomain "github.com/korwlik23/go-api-starter/internal/modules/publishing/domain"
	"github.com/korwlik23/go-api-starter/internal/modules/publishing/ports"
)

type contentListResponse struct {
	Items []contentListItemResponse `json:"items"`
	Next  *contentCursorResponse    `json:"next,omitempty"`
}

type contentListItemResponse struct {
	ContentID          string                  `json:"content_id"`
	AccountID          string                  `json:"account_id"`
	LocaleID           string                  `json:"locale_id"`
	Kind               publishingdomain.Kind   `json:"kind"`
	ContentKey         string                  `json:"content_key"`
	ContentStatus      publishingdomain.Status `json:"content_status"`
	ContentVersion     int64                   `json:"content_version"`
	TranslationID      string                  `json:"translation_id"`
	TranslationStatus  publishingdomain.Status `json:"translation_status"`
	TranslationVersion int64                   `json:"translation_version"`
	Slug               string                  `json:"slug"`
	Path               string                  `json:"path"`
	Title              string                  `json:"title"`
	UpdatedAt          string                  `json:"updated_at"`
}

type contentCursorResponse struct {
	UpdatedAt string `json:"updated_at"`
	ID        string `json:"id"`
}

func (handler *ContentHandler) ListPages(ctx *gin.Context) {
	handler.list(ctx, publishingdomain.KindPage)
}

func (handler *ContentHandler) ListPosts(ctx *gin.Context) {
	handler.list(ctx, publishingdomain.KindPost)
}

func (handler *ContentHandler) GetPage(ctx *gin.Context) {
	handler.get(ctx, publishingdomain.KindPage)
}

func (handler *ContentHandler) GetPost(ctx *gin.Context) {
	handler.get(ctx, publishingdomain.KindPost)
}

func (handler *ContentHandler) list(ctx *gin.Context, kind publishingdomain.Kind) {
	_, accountID, ok := handler.principal(ctx)
	if !ok || handler.queries == nil {
		if ok {
			writeContentProblem(ctx, 501, contentapp.ErrInvalidContentQuery)
		}
		return
	}
	locale := strings.TrimSpace(ctx.Query("locale"))
	if locale == "" {
		writeContentProblem(ctx, 400, contentapp.ErrInvalidContentQuery)
		return
	}
	limit, err := parseContentLimit(ctx.Query("limit"))
	if err != nil {
		writeContentProblem(ctx, 400, err)
		return
	}
	var after *ports.ContentCursor
	if afterID := strings.TrimSpace(ctx.Query("after_id")); afterID != "" {
		afterUpdatedAt, parseErr := time.Parse(time.RFC3339Nano, strings.TrimSpace(ctx.Query("after_updated_at")))
		if parseErr != nil {
			writeContentProblem(ctx, 400, contentapp.ErrInvalidContentQuery)
			return
		}
		after = &ports.ContentCursor{ID: afterID, UpdatedAt: afterUpdatedAt}
	}
	result, err := handler.queries.List(ctx.Request.Context(), contentapp.ListInput{
		AccountID: accountID, Locale: locale, Kind: &kind, After: after, Limit: limit,
	})
	if err != nil {
		writeContentQueryError(ctx, err)
		return
	}
	ctx.JSON(200, toContentListResponse(result))
}

func (handler *ContentHandler) get(ctx *gin.Context, kind publishingdomain.Kind) {
	_, accountID, ok := handler.principal(ctx)
	if !ok || handler.queries == nil {
		if ok {
			writeContentProblem(ctx, 501, contentapp.ErrInvalidContentQuery)
		}
		return
	}
	locale := strings.TrimSpace(ctx.Query("locale"))
	if locale == "" {
		writeContentProblem(ctx, 400, contentapp.ErrInvalidContentQuery)
		return
	}
	detail, err := handler.queries.Get(ctx.Request.Context(), contentapp.GetInput{
		AccountID: accountID, ContentID: contentIDParam(ctx), Locale: locale,
	})
	if err != nil {
		writeContentQueryError(ctx, err)
		return
	}
	if detail.Kind != kind {
		writeContentProblem(ctx, 404, ports.ErrContentNotFound)
		return
	}
	ctx.JSON(200, toContentResponse(detail))
}

func parseContentLimit(raw string) (int, error) {
	if strings.TrimSpace(raw) == "" {
		return 50, nil
	}
	limit, err := strconv.Atoi(raw)
	if err != nil || limit < 1 || limit > 100 {
		return 0, contentapp.ErrInvalidContentQuery
	}
	return limit, nil
}

func toContentListResponse(page ports.ContentListPage) contentListResponse {
	items := make([]contentListItemResponse, 0, len(page.Items))
	for _, item := range page.Items {
		items = append(items, contentListItemResponse{
			ContentID: item.ContentID, AccountID: item.AccountID, LocaleID: item.LocaleID,
			Kind: item.Kind, ContentKey: item.ContentKey, ContentStatus: item.ContentStatus,
			ContentVersion: item.ContentVersion, TranslationID: item.TranslationID,
			TranslationStatus: item.TranslationStatus, TranslationVersion: item.TranslationVersion,
			Slug: item.Slug, Path: item.Path, Title: item.Title,
			UpdatedAt: item.UpdatedAt.UTC().Format(time.RFC3339Nano),
		})
	}
	result := contentListResponse{Items: items}
	if page.Next != nil {
		result.Next = &contentCursorResponse{
			ID: page.Next.ID, UpdatedAt: page.Next.UpdatedAt.UTC().Format(time.RFC3339Nano),
		}
	}
	return result
}

func writeContentQueryError(ctx *gin.Context, err error) {
	status := 500
	switch {
	case errors.Is(err, contentapp.ErrInvalidContentQuery), errors.Is(err, ports.ErrInvalidContentQuery):
		status = 400
	case errors.Is(err, ports.ErrContentNotFound):
		status = 404
	}
	writeContentProblem(ctx, status, err)
}
