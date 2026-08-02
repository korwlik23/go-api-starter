package http

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	redirectsapp "github.com/korwlik23/go-api-starter/internal/modules/discoverability/application/redirects"
	discoverabilitydomain "github.com/korwlik23/go-api-starter/internal/modules/discoverability/domain"
	"github.com/korwlik23/go-api-starter/internal/modules/discoverability/ports"
	platformmiddleware "github.com/korwlik23/go-api-starter/internal/platform/middleware"
)

var ErrInvalidRedirectHandler = errors.New("invalid discoverability redirect handler dependencies")

type redirectManager interface {
	List(context.Context, redirectsapp.ListInput) (ports.RedirectListPage, error)
	Get(context.Context, string, string) (discoverabilitydomain.Redirect, error)
	Create(context.Context, redirectsapp.CreateInput) (discoverabilitydomain.Redirect, error)
	Update(context.Context, redirectsapp.UpdateInput) (discoverabilitydomain.Redirect, error)
	Delete(context.Context, redirectsapp.DeleteInput) error
}

type RedirectHandler struct {
	manager  redirectManager
	accounts accountResolver
}

func NewRedirectHandler(manager redirectManager, accounts accountResolver) (*RedirectHandler, error) {
	if manager == nil || accounts == nil {
		return nil, ErrInvalidRedirectHandler
	}
	return &RedirectHandler{manager: manager, accounts: accounts}, nil
}

type redirectMutationRequest struct {
	LocaleID        string `json:"locale_id"`
	SourcePath      string `json:"source_path"`
	DestinationPath string `json:"destination_path"`
	StatusCode      int    `json:"status_code"`
	Enabled         bool   `json:"enabled"`
}
type redirectUpdateRequest struct {
	redirectMutationRequest
	ExpectedVersion int64 `json:"expected_version"`
}
type redirectDeleteRequest struct {
	ExpectedVersion int64 `json:"expected_version"`
}
type redirectResponse struct {
	ID              string `json:"id"`
	AccountID       string `json:"account_id"`
	LocaleID        string `json:"locale_id"`
	SourcePath      string `json:"source_path"`
	DestinationPath string `json:"destination_path"`
	StatusCode      int    `json:"status_code"`
	Enabled         bool   `json:"enabled"`
	Version         int64  `json:"version"`
	CreatedAt       string `json:"created_at"`
	UpdatedAt       string `json:"updated_at"`
}
type redirectListResponse struct {
	Items []redirectResponse      `json:"items"`
	Next  *redirectCursorResponse `json:"next,omitempty"`
}
type redirectCursorResponse struct {
	UpdatedAt string `json:"updated_at"`
	ID        string `json:"id"`
}

func (handler *RedirectHandler) List(ctx *gin.Context) {
	_, accountID, ok := principal(ctx, handler.accounts)
	if !ok {
		return
	}
	limit := 25
	if raw := strings.TrimSpace(ctx.Query("limit")); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 1 || parsed > 100 {
			writeProblem(ctx, http.StatusBadRequest, "invalid_request", "The redirect list request is invalid.", nil)
			return
		}
		limit = parsed
	}
	var cursor *ports.RedirectCursor
	if rawUpdated, rawID := strings.TrimSpace(ctx.Query("after_updated_at")), strings.TrimSpace(ctx.Query("after_id")); rawUpdated != "" || rawID != "" {
		updated, err := time.Parse(time.RFC3339Nano, rawUpdated)
		if err != nil || rawID == "" {
			writeProblem(ctx, http.StatusBadRequest, "invalid_request", "The redirect cursor is invalid.", nil)
			return
		}
		cursor = &ports.RedirectCursor{UpdatedAt: updated, ID: rawID}
	}
	var enabled *bool
	if raw := strings.TrimSpace(ctx.Query("enabled")); raw != "" {
		parsed, err := strconv.ParseBool(raw)
		if err != nil {
			writeProblem(ctx, http.StatusBadRequest, "invalid_request", "The redirect filter is invalid.", nil)
			return
		}
		enabled = &parsed
	}
	page, err := handler.manager.List(ctx.Request.Context(), redirectsapp.ListInput{AccountID: accountID, LocaleID: strings.TrimSpace(ctx.Query("locale_id")), Enabled: enabled, After: cursor, Limit: limit})
	if err != nil {
		writeDiscoverabilityError(ctx, err)
		return
	}
	items := make([]redirectResponse, 0, len(page.Items))
	for _, item := range page.Items {
		items = append(items, toRedirectResponse(item))
	}
	result := redirectListResponse{Items: items}
	if page.Next != nil {
		result.Next = &redirectCursorResponse{UpdatedAt: page.Next.UpdatedAt.UTC().Format(time.RFC3339Nano), ID: page.Next.ID}
	}
	ctx.Header("Cache-Control", "no-store")
	ctx.JSON(http.StatusOK, result)
}

func (handler *RedirectHandler) Get(ctx *gin.Context) {
	_, accountID, ok := principal(ctx, handler.accounts)
	if !ok {
		return
	}
	redirect, err := handler.manager.Get(ctx.Request.Context(), accountID, strings.TrimSpace(ctx.Param("redirect_id")))
	if err != nil {
		writeDiscoverabilityError(ctx, err)
		return
	}
	ctx.Header("Cache-Control", "no-store")
	ctx.JSON(http.StatusOK, toRedirectResponse(redirect))
}

func (handler *RedirectHandler) Create(ctx *gin.Context) {
	userID, accountID, ok := principal(ctx, handler.accounts)
	if !ok || !prepareJSON(ctx) {
		return
	}
	var request redirectMutationRequest
	if !decodeJSON(ctx, &request) {
		return
	}
	redirect, err := handler.manager.Create(ctx.Request.Context(), redirectsapp.CreateInput{AccountID: accountID, ActorUserID: userID, RequestID: platformmiddleware.RequestIDFromContext(ctx.Request.Context()), LocaleID: request.LocaleID, SourcePath: request.SourcePath, DestinationPath: request.DestinationPath, StatusCode: discoverabilitydomain.StatusCode(request.StatusCode), Enabled: request.Enabled})
	if err != nil {
		writeDiscoverabilityError(ctx, err)
		return
	}
	ctx.Header("Cache-Control", "no-store")
	ctx.JSON(http.StatusCreated, toRedirectResponse(redirect))
}

func (handler *RedirectHandler) Update(ctx *gin.Context) {
	userID, accountID, ok := principal(ctx, handler.accounts)
	if !ok || !prepareJSON(ctx) {
		return
	}
	var request redirectUpdateRequest
	if !decodeJSON(ctx, &request) {
		return
	}
	redirect, err := handler.manager.Update(ctx.Request.Context(), redirectsapp.UpdateInput{AccountID: accountID, ActorUserID: userID, RequestID: platformmiddleware.RequestIDFromContext(ctx.Request.Context()), RedirectID: strings.TrimSpace(ctx.Param("redirect_id")), ExpectedVersion: request.ExpectedVersion, LocaleID: request.LocaleID, SourcePath: request.SourcePath, DestinationPath: request.DestinationPath, StatusCode: discoverabilitydomain.StatusCode(request.StatusCode), Enabled: request.Enabled})
	if err != nil {
		writeDiscoverabilityError(ctx, err)
		return
	}
	ctx.Header("Cache-Control", "no-store")
	ctx.JSON(http.StatusOK, toRedirectResponse(redirect))
}

func (handler *RedirectHandler) Delete(ctx *gin.Context) {
	userID, accountID, ok := principal(ctx, handler.accounts)
	if !ok || !prepareJSON(ctx) {
		return
	}
	var request redirectDeleteRequest
	if !decodeJSON(ctx, &request) {
		return
	}
	if err := handler.manager.Delete(ctx.Request.Context(), redirectsapp.DeleteInput{AccountID: accountID, ActorUserID: userID, RequestID: platformmiddleware.RequestIDFromContext(ctx.Request.Context()), RedirectID: strings.TrimSpace(ctx.Param("redirect_id")), ExpectedVersion: request.ExpectedVersion}); err != nil {
		writeDiscoverabilityError(ctx, err)
		return
	}
	ctx.Status(http.StatusNoContent)
}

func toRedirectResponse(redirect discoverabilitydomain.Redirect) redirectResponse {
	return redirectResponse{ID: redirect.ID, AccountID: redirect.AccountID, LocaleID: redirect.LocaleID, SourcePath: redirect.SourcePath, DestinationPath: redirect.DestinationPath, StatusCode: int(redirect.StatusCode), Enabled: redirect.Enabled, Version: redirect.Version, CreatedAt: redirect.CreatedAt.UTC().Format(time.RFC3339Nano), UpdatedAt: redirect.UpdatedAt.UTC().Format(time.RFC3339Nano)}
}
