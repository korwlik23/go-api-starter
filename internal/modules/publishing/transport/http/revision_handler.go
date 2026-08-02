package http

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	identitydomain "github.com/korwlik23/go-api-starter/internal/modules/identity/domain"
	revisionsapp "github.com/korwlik23/go-api-starter/internal/modules/publishing/application/revisions"
	"github.com/korwlik23/go-api-starter/internal/modules/publishing/ports"
	platformmiddleware "github.com/korwlik23/go-api-starter/internal/platform/middleware"
	"github.com/korwlik23/go-api-starter/internal/platform/response"
)

var ErrInvalidRevisionHandler = errors.New("invalid revision handler dependencies")

type revisionReader interface {
	List(context.Context, revisionsapp.ListInput) (ports.RevisionPage, error)
}

type revisionManager interface {
	Rollback(context.Context, revisionsapp.RollbackInput) (revisionsapp.RollbackResult, error)
}

type RevisionHandler struct {
	reader   revisionReader
	manager  revisionManager
	accounts accountResolver
}

func NewRevisionHandler(
	reader revisionReader,
	manager revisionManager,
	accounts accountResolver,
) (*RevisionHandler, error) {
	if reader == nil || manager == nil || accounts == nil {
		return nil, ErrInvalidRevisionHandler
	}
	return &RevisionHandler{reader: reader, manager: manager, accounts: accounts}, nil
}

type revisionListResponse struct {
	Items []revisionResponse      `json:"items"`
	Next  *revisionCursorResponse `json:"next,omitempty"`
}

type revisionResponse struct {
	ID            string    `json:"id"`
	ContentID     string    `json:"content_id"`
	TranslationID string    `json:"translation_id"`
	AuthorID      string    `json:"author_id"`
	SnapshotHash  string    `json:"snapshot_hash"`
	ChangeSummary string    `json:"change_summary"`
	SourceVersion int64     `json:"source_version"`
	CreatedAt     time.Time `json:"created_at"`
}

type revisionCursorResponse struct {
	CreatedAt time.Time `json:"created_at"`
	ID        string    `json:"id"`
}

type revisionRollbackRequest struct {
	TranslationID   string `json:"translation_id"`
	LocaleID        string `json:"locale_id"`
	ExpectedVersion int64  `json:"expected_version"`
	ChangeSummary   string `json:"change_summary"`
}

type revisionRollbackResponse struct {
	Revision revisionResponse `json:"revision"`
	Content  struct {
		ContentID          string `json:"content_id"`
		TranslationID      string `json:"translation_id"`
		ContentVersion     int64  `json:"content_version"`
		TranslationVersion int64  `json:"translation_version"`
		Status             string `json:"status"`
	} `json:"content"`
}

func (handler *RevisionHandler) List(ctx *gin.Context) {
	userID, accountID, ok := handler.principal(ctx)
	if !ok {
		return
	}
	_ = userID
	if err := rejectRevisionQueryKeys(ctx); err != nil {
		writeRevisionProblem(ctx, http.StatusBadRequest, "invalid_request", "The revision list request is invalid.", err)
		return
	}
	translationID := strings.TrimSpace(ctx.Query("translation_id"))
	limit, err := parseRevisionLimit(ctx)
	if err != nil || translationID == "" {
		writeRevisionProblem(ctx, http.StatusBadRequest, "invalid_request", "The revision list request is invalid.", err)
		return
	}
	var after *ports.RevisionCursor
	if cursorID := strings.TrimSpace(ctx.Query("after_id")); cursorID != "" {
		createdAt, parseErr := time.Parse(time.RFC3339Nano, strings.TrimSpace(ctx.Query("after_created_at")))
		if parseErr != nil {
			writeRevisionProblem(ctx, http.StatusBadRequest, "invalid_request", "The revision list cursor is invalid.", parseErr)
			return
		}
		after = &ports.RevisionCursor{CreatedAt: createdAt, ID: cursorID}
	}
	page, err := handler.reader.List(ctx.Request.Context(), revisionsapp.ListInput{
		AccountID: accountID, ContentID: contentIDParam(ctx),
		TranslationID: translationID, After: after, Limit: limit,
	})
	if err != nil {
		writeRevisionError(ctx, err, "The revisions could not be listed.")
		return
	}
	items := make([]revisionResponse, 0, len(page.Items))
	for _, item := range page.Items {
		items = append(items, revisionResponseFromView(item))
	}
	var next *revisionCursorResponse
	if page.Next != nil {
		next = &revisionCursorResponse{CreatedAt: page.Next.CreatedAt, ID: page.Next.ID}
	}
	ctx.Header("Cache-Control", "no-store")
	ctx.JSON(http.StatusOK, revisionListResponse{Items: items, Next: next})
}

func (handler *RevisionHandler) Rollback(ctx *gin.Context) {
	userID, accountID, ok := handler.principal(ctx)
	if !ok {
		return
	}
	if strings.TrimSpace(platformmiddleware.RequestIDFromContext(ctx.Request.Context())) == "" {
		writeRevisionProblem(ctx, http.StatusBadRequest, "invalid_request", "A request ID is required for revision mutations.", nil)
		return
	}
	if err := requireJSONContentType(ctx); err != nil {
		writeRevisionProblem(ctx, http.StatusUnsupportedMediaType, "unsupported_media_type", "Revision mutations require application/json.", err)
		return
	}
	var request revisionRollbackRequest
	if err := decodeStrictJSON(ctx, &request); err != nil {
		writeRevisionProblem(ctx, http.StatusBadRequest, "invalid_request", "The revision rollback request is invalid.", err)
		return
	}
	result, err := handler.manager.Rollback(ctx.Request.Context(), revisionsapp.RollbackInput{
		AccountID: accountID, ActorUserID: userID,
		RequestID: platformmiddleware.RequestIDFromContext(ctx.Request.Context()),
		ContentID: contentIDParam(ctx), RevisionID: strings.TrimSpace(ctx.Param("revision_id")),
		TranslationID: request.TranslationID, LocaleID: request.LocaleID,
		ExpectedVersion: request.ExpectedVersion, ChangeSummary: request.ChangeSummary,
	})
	if err != nil {
		writeRevisionError(ctx, err, "The revision could not be rolled back.")
		return
	}
	body := revisionRollbackResponse{Revision: revisionResponseFromView(result.Revision)}
	body.Content.ContentID = result.Content.ContentID
	body.Content.TranslationID = result.Content.TranslationID
	body.Content.ContentVersion = result.Content.ContentVersion
	body.Content.TranslationVersion = result.Content.TranslationVersion
	body.Content.Status = string(result.Content.TranslationStatus)
	ctx.Header("Cache-Control", "no-store")
	ctx.JSON(http.StatusOK, body)
}

func (handler *RevisionHandler) principal(ctx *gin.Context) (string, string, bool) {
	session, ok := platformmiddleware.SessionFromContext(ctx.Request.Context())
	if !ok || session.State != identitydomain.SessionStateAuthenticated ||
		session.UserID == nil || strings.TrimSpace(*session.UserID) == "" {
		response.WriteProblem(ctx, http.StatusUnauthorized, platformmiddleware.RequestIDFromContext(ctx.Request.Context()), response.NewProblem("authentication_required", "Authentication is required.", nil, nil))
		return "", "", false
	}
	resolved, err := handler.accounts.Execute(ctx.Request.Context(), *session.UserID)
	if err != nil || resolved.Account.ID == "" {
		response.WriteProblem(ctx, http.StatusForbidden, platformmiddleware.RequestIDFromContext(ctx.Request.Context()), response.NewProblem("permission_denied", "Permission is required.", nil, err))
		return "", "", false
	}
	return *session.UserID, resolved.Account.ID, true
}

func parseRevisionLimit(ctx *gin.Context) (int, error) {
	value := strings.TrimSpace(ctx.Query("limit"))
	if value == "" {
		return 25, nil
	}
	limit, err := strconv.Atoi(value)
	if err != nil || limit < 1 || limit > 100 {
		return 0, errors.New("limit must be between 1 and 100")
	}
	return limit, nil
}

func rejectRevisionQueryKeys(ctx *gin.Context) error {
	allowed := map[string]bool{
		"translation_id": true, "limit": true,
		"after_created_at": true, "after_id": true,
	}
	for key, values := range ctx.Request.URL.Query() {
		if !allowed[key] || len(values) != 1 || strings.TrimSpace(values[0]) == "" {
			return errors.New("invalid query parameter: " + key)
		}
	}
	if _, hasCreatedAt := ctx.GetQuery("after_created_at"); hasCreatedAt {
		if _, hasID := ctx.GetQuery("after_id"); !hasID {
			return errors.New("after_id is required with after_created_at")
		}
	}
	if _, hasID := ctx.GetQuery("after_id"); hasID {
		if _, hasCreatedAt := ctx.GetQuery("after_created_at"); !hasCreatedAt {
			return errors.New("after_created_at is required with after_id")
		}
	}
	return nil
}

func revisionResponseFromView(view ports.RevisionView) revisionResponse {
	return revisionResponse{
		ID: view.Revision.ID, ContentID: view.Revision.ContentID,
		TranslationID: view.Revision.TranslationID, AuthorID: view.Revision.AuthorID,
		SnapshotHash: view.Revision.SnapshotHash, ChangeSummary: view.Revision.ChangeSummary,
		SourceVersion: view.Revision.SourceVersion, CreatedAt: view.Revision.CreatedAt,
	}
}

func writeRevisionError(ctx *gin.Context, err error, message string) {
	switch {
	case errors.Is(err, ports.ErrRevisionConflict), errors.Is(err, ports.ErrContentConflict):
		writeRevisionProblem(ctx, http.StatusConflict, "content_version_conflict", "The content changed before the revision operation completed.", nil)
	case errors.Is(err, ports.ErrRevisionNotFound), errors.Is(err, ports.ErrContentNotFound):
		writeRevisionProblem(ctx, http.StatusNotFound, "not_found", "The revision was not found.", nil)
	case errors.Is(err, ports.ErrInvalidRevisionQuery), errors.Is(err, revisionsapp.ErrInvalidRevisionService):
		writeRevisionProblem(ctx, http.StatusBadRequest, "invalid_request", "The revision request is invalid.", nil)
	default:
		writeRevisionProblem(ctx, http.StatusInternalServerError, "internal_error", message, err)
	}
}

func writeRevisionProblem(ctx *gin.Context, status int, code, message string, cause error) {
	response.WriteProblem(ctx, status, platformmiddleware.RequestIDFromContext(ctx.Request.Context()), response.NewProblem(code, message, nil, cause))
}

var _ revisionReader = (*revisionsapp.Service)(nil)
var _ revisionManager = (*revisionsapp.Service)(nil)
