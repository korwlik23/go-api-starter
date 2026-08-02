package http

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	identitydomain "github.com/korwlik23/go-api-starter/internal/modules/identity/domain"
	previewapp "github.com/korwlik23/go-api-starter/internal/modules/publishing/application/preview"
	"github.com/korwlik23/go-api-starter/internal/modules/publishing/ports"
	platformmiddleware "github.com/korwlik23/go-api-starter/internal/platform/middleware"
	"github.com/korwlik23/go-api-starter/internal/platform/response"
)

var ErrInvalidPreviewHandler = errors.New("invalid preview handler dependencies")

type previewIssueReader interface {
	Execute(context.Context, previewapp.IssueInput) (previewapp.IssueResult, error)
}

type previewExchangeReader interface {
	Execute(context.Context, string) (previewapp.ExchangeResult, error)
}

type PreviewHandler struct {
	issue    previewIssueReader
	exchange previewExchangeReader
	accounts accountResolver
}

func NewPreviewHandler(
	issue previewIssueReader,
	exchange previewExchangeReader,
	accounts accountResolver,
) (*PreviewHandler, error) {
	if issue == nil || exchange == nil || accounts == nil {
		return nil, ErrInvalidPreviewHandler
	}
	return &PreviewHandler{issue: issue, exchange: exchange, accounts: accounts}, nil
}

type previewIssueRequest struct {
	TranslationID string `json:"translation_id"`
	RevisionID    string `json:"revision_id"`
}

type previewIssueResponse struct {
	Token      string    `json:"token"`
	TokenID    string    `json:"token_id"`
	RevisionID string    `json:"revision_id"`
	ExpiresAt  time.Time `json:"expires_at"`
}

type previewExchangeRequest struct {
	Token string `json:"token"`
}

type previewExchangeResponse struct {
	RevisionID    string          `json:"revision_id"`
	ContentID     string          `json:"content_id"`
	TranslationID string          `json:"translation_id"`
	SourceVersion int64           `json:"source_version"`
	Snapshot      json.RawMessage `json:"snapshot"`
	ExpiresAt     time.Time       `json:"expires_at"`
}

func (handler *PreviewHandler) Issue(ctx *gin.Context) {
	userID, accountID, ok := handler.principal(ctx)
	if !ok {
		return
	}
	if strings.TrimSpace(platformmiddleware.RequestIDFromContext(ctx.Request.Context())) == "" {
		writePreviewProblem(ctx, http.StatusBadRequest, "invalid_request", "A request ID is required for preview issue.")
		return
	}
	if err := requireJSONContentType(ctx); err != nil {
		writePreviewProblem(ctx, http.StatusUnsupportedMediaType, "unsupported_media_type", "Preview issue requires application/json.")
		return
	}
	var request previewIssueRequest
	if err := decodeStrictJSON(ctx, &request); err != nil {
		writePreviewProblem(ctx, http.StatusBadRequest, "invalid_request", "The preview issue request is invalid.")
		return
	}
	result, err := handler.issue.Execute(ctx.Request.Context(), previewapp.IssueInput{
		AccountID: accountID, UserID: userID, ContentID: strings.TrimSpace(ctx.Param("content_id")),
		TranslationID: strings.TrimSpace(request.TranslationID), RevisionID: strings.TrimSpace(request.RevisionID),
	})
	if err != nil {
		writePreviewIssueError(ctx, err)
		return
	}
	setPreviewNoStoreHeaders(ctx)
	ctx.JSON(http.StatusCreated, previewIssueResponse{
		Token: result.RawToken, TokenID: result.TokenID,
		RevisionID: result.RevisionID, ExpiresAt: result.ExpiresAt,
	})
}

func (handler *PreviewHandler) Exchange(ctx *gin.Context) {
	if err := requireJSONContentType(ctx); err != nil {
		setPreviewNoStoreHeaders(ctx)
		writePreviewProblem(ctx, http.StatusUnsupportedMediaType, "unsupported_media_type", "Preview exchange requires application/json.")
		return
	}
	var request previewExchangeRequest
	if err := decodeStrictJSON(ctx, &request); err != nil || strings.TrimSpace(request.Token) == "" {
		setPreviewNoStoreHeaders(ctx)
		writePreviewProblem(ctx, http.StatusNotFound, "preview_not_found", "The preview was not found.")
		return
	}
	result, err := handler.exchange.Execute(ctx.Request.Context(), request.Token)
	if err != nil {
		setPreviewNoStoreHeaders(ctx)
		writePreviewProblem(ctx, http.StatusNotFound, "preview_not_found", "The preview was not found.")
		return
	}
	setPreviewNoStoreHeaders(ctx)
	ctx.JSON(http.StatusOK, previewExchangeResponse{
		RevisionID:    result.Revision.Revision.ID,
		ContentID:     result.Revision.Revision.ContentID,
		TranslationID: result.Revision.Revision.TranslationID,
		SourceVersion: result.Revision.Revision.SourceVersion,
		Snapshot:      append(json.RawMessage(nil), result.Revision.Snapshot...),
		ExpiresAt:     result.ExpiresAt,
	})
}

func (handler *PreviewHandler) principal(ctx *gin.Context) (string, string, bool) {
	session, ok := platformmiddleware.SessionFromContext(ctx.Request.Context())
	if !ok || session.State != identitydomain.SessionStateAuthenticated ||
		session.UserID == nil || strings.TrimSpace(*session.UserID) == "" {
		response.WriteProblem(ctx, http.StatusUnauthorized, platformmiddleware.RequestIDFromContext(ctx.Request.Context()), response.NewProblem("authentication_required", "Authentication is required.", nil, nil))
		return "", "", false
	}
	resolved, err := handler.accounts.Execute(ctx.Request.Context(), *session.UserID)
	if err != nil || strings.TrimSpace(resolved.Account.ID) == "" {
		response.WriteProblem(ctx, http.StatusForbidden, platformmiddleware.RequestIDFromContext(ctx.Request.Context()), response.NewProblem("permission_denied", "Permission is required.", nil, err))
		return "", "", false
	}
	return *session.UserID, resolved.Account.ID, true
}

func setPreviewNoStoreHeaders(ctx *gin.Context) {
	ctx.Header("Cache-Control", "no-store")
	ctx.Header("X-Robots-Tag", "noindex, nofollow")
}

func writePreviewIssueError(ctx *gin.Context, err error) {
	switch {
	case errors.Is(err, ports.ErrPreviewNotFound):
		writePreviewProblem(ctx, http.StatusNotFound, "preview_not_found", "The preview was not found.")
	case errors.Is(err, previewapp.ErrInvalidPreviewIssue):
		writePreviewProblem(ctx, http.StatusUnprocessableEntity, "invalid_request", "The preview issue request is invalid.")
	default:
		writePreviewProblem(ctx, http.StatusInternalServerError, "internal_error", "The preview could not be issued.")
	}
}

func writePreviewProblem(ctx *gin.Context, status int, code, message string) {
	response.WriteProblem(
		ctx, status, platformmiddleware.RequestIDFromContext(ctx.Request.Context()),
		response.NewProblem(code, message, nil, nil),
	)
}
