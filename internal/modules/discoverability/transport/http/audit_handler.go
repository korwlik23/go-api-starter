package http

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	auditapp "github.com/korwlik23/go-api-starter/internal/modules/discoverability/application/audit"
	discoverabilitydomain "github.com/korwlik23/go-api-starter/internal/modules/discoverability/domain"
	"github.com/korwlik23/go-api-starter/internal/modules/discoverability/ports"
	publishingports "github.com/korwlik23/go-api-starter/internal/modules/publishing/ports"
)

var ErrInvalidAuditHandler = errors.New("invalid discoverability audit handler dependencies")

type auditManager interface {
	Run(context.Context, auditapp.RunInput) (discoverabilitydomain.ContentAudit, error)
	Get(context.Context, string, string) (discoverabilitydomain.ContentAudit, error)
	List(context.Context, auditapp.ListInput) (ports.ContentAuditListPage, error)
}

type AuditHandler struct {
	manager  auditManager
	accounts accountResolver
}

func NewAuditHandler(manager auditManager, accounts accountResolver) (*AuditHandler, error) {
	if manager == nil || accounts == nil {
		return nil, ErrInvalidAuditHandler
	}
	return &AuditHandler{manager: manager, accounts: accounts}, nil
}

type auditRunRequest struct {
	ContentID string `json:"content_id"`
	LocaleID  string `json:"locale_id"`
}

type auditResponse struct {
	ID                 string                             `json:"id"`
	AccountID          string                             `json:"account_id"`
	ContentID          string                             `json:"content_id"`
	TranslationID      string                             `json:"translation_id"`
	LocaleID           string                             `json:"locale_id"`
	ContentVersion     int64                              `json:"content_version"`
	TranslationVersion int64                              `json:"translation_version"`
	Status             discoverabilitydomain.AuditStatus  `json:"status"`
	Checks             []discoverabilitydomain.AuditCheck `json:"checks"`
	CreatedAt          string                             `json:"created_at"`
	UpdatedAt          string                             `json:"updated_at"`
}

type auditListResponse struct {
	Items []auditResponse      `json:"items"`
	Next  *auditCursorResponse `json:"next,omitempty"`
}

type auditCursorResponse struct {
	UpdatedAt string `json:"updated_at"`
	ID        string `json:"id"`
}

func (handler *AuditHandler) Run(ctx *gin.Context) {
	_, accountID, ok := principal(ctx, handler.accounts)
	if !ok || !prepareJSON(ctx) {
		return
	}
	var request auditRunRequest
	if !decodeJSON(ctx, &request) {
		return
	}
	result, err := handler.manager.Run(ctx.Request.Context(), auditapp.RunInput{AccountID: accountID, ContentID: strings.TrimSpace(request.ContentID), LocaleID: strings.TrimSpace(request.LocaleID)})
	if err != nil {
		writeAuditError(ctx, err)
		return
	}
	ctx.Header("Cache-Control", "no-store")
	ctx.JSON(http.StatusCreated, toAuditResponse(result))
}

func (handler *AuditHandler) Get(ctx *gin.Context) {
	_, accountID, ok := principal(ctx, handler.accounts)
	if !ok {
		return
	}
	result, err := handler.manager.Get(ctx.Request.Context(), accountID, strings.TrimSpace(ctx.Param("audit_id")))
	if err != nil {
		writeAuditError(ctx, err)
		return
	}
	ctx.Header("Cache-Control", "no-store")
	ctx.JSON(http.StatusOK, toAuditResponse(result))
}

func (handler *AuditHandler) List(ctx *gin.Context) {
	_, accountID, ok := principal(ctx, handler.accounts)
	if !ok {
		return
	}
	limit := 25
	if raw := strings.TrimSpace(ctx.Query("limit")); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 1 || parsed > 100 {
			writeProblem(ctx, http.StatusBadRequest, "invalid_request", "The audit list request is invalid.", nil)
			return
		}
		limit = parsed
	}
	var cursor *ports.ContentAuditCursor
	if rawUpdated, rawID := strings.TrimSpace(ctx.Query("after_updated_at")), strings.TrimSpace(ctx.Query("after_id")); rawUpdated != "" || rawID != "" {
		updated, err := time.Parse(time.RFC3339Nano, rawUpdated)
		if err != nil || rawID == "" {
			writeProblem(ctx, http.StatusBadRequest, "invalid_request", "The audit cursor is invalid.", nil)
			return
		}
		cursor = &ports.ContentAuditCursor{UpdatedAt: updated, ID: rawID}
	}
	var status *discoverabilitydomain.AuditStatus
	if raw := strings.TrimSpace(ctx.Query("status")); raw != "" {
		value := discoverabilitydomain.AuditStatus(raw)
		status = &value
	}
	page, err := handler.manager.List(ctx.Request.Context(), auditapp.ListInput{AccountID: accountID, ContentID: strings.TrimSpace(ctx.Query("content_id")), TranslationID: strings.TrimSpace(ctx.Query("translation_id")), LocaleID: strings.TrimSpace(ctx.Query("locale_id")), Status: status, After: cursor, Limit: limit})
	if err != nil {
		writeAuditError(ctx, err)
		return
	}
	result := auditListResponse{Items: make([]auditResponse, 0, len(page.Items))}
	for _, audit := range page.Items {
		result.Items = append(result.Items, toAuditResponse(audit))
	}
	if page.Next != nil {
		result.Next = &auditCursorResponse{UpdatedAt: page.Next.UpdatedAt.UTC().Format(time.RFC3339Nano), ID: page.Next.ID}
	}
	ctx.Header("Cache-Control", "no-store")
	ctx.JSON(http.StatusOK, result)
}

func toAuditResponse(audit discoverabilitydomain.ContentAudit) auditResponse {
	return auditResponse{ID: audit.ID, AccountID: audit.AccountID, ContentID: audit.ContentID, TranslationID: audit.TranslationID, LocaleID: audit.LocaleID, ContentVersion: audit.ContentVersion, TranslationVersion: audit.TranslationVersion, Status: audit.Status, Checks: audit.Checks, CreatedAt: audit.CreatedAt.UTC().Format(time.RFC3339Nano), UpdatedAt: audit.UpdatedAt.UTC().Format(time.RFC3339Nano)}
}

func writeAuditError(ctx *gin.Context, err error) {
	switch {
	case errors.Is(err, ports.ErrContentAuditNotFound), errors.Is(err, publishingports.ErrContentNotFound):
		writeProblem(ctx, http.StatusNotFound, "not_found", "The content audit was not found.", nil)
	case errors.Is(err, ports.ErrContentAuditConflict):
		writeProblem(ctx, http.StatusConflict, "conflict", "The content audit changed; reload before retrying.", nil)
	case errors.Is(err, ports.ErrInvalidContentAuditQuery):
		writeProblem(ctx, http.StatusBadRequest, "invalid_request", "The content audit query is invalid.", nil)
	case errors.Is(err, auditapp.ErrInvalidAuditSubject), errors.Is(err, discoverabilitydomain.ErrInvalidContentAudit):
		writeProblem(ctx, http.StatusUnprocessableEntity, "invalid_request", "The content audit request is invalid.", nil)
	default:
		writeProblem(ctx, http.StatusInternalServerError, "internal_error", "The content audit request could not be completed.", nil)
	}
}
