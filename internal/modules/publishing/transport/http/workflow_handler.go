package http

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	identitydomain "github.com/korwlik23/go-api-starter/internal/modules/identity/domain"
	workflowapp "github.com/korwlik23/go-api-starter/internal/modules/publishing/application/workflow"
	publishingdomain "github.com/korwlik23/go-api-starter/internal/modules/publishing/domain"
	"github.com/korwlik23/go-api-starter/internal/modules/publishing/ports"
	platformmiddleware "github.com/korwlik23/go-api-starter/internal/platform/middleware"
	"github.com/korwlik23/go-api-starter/internal/platform/response"
)

var ErrInvalidWorkflowHandler = errors.New("invalid workflow handler dependencies")

type workflowManager interface {
	Transition(context.Context, workflowapp.TransitionInput) (ports.ContentDetail, error)
}

type WorkflowHandler struct {
	manager  workflowManager
	accounts accountResolver
}

func NewWorkflowHandler(manager workflowManager, accounts accountResolver) (*WorkflowHandler, error) {
	if manager == nil || accounts == nil {
		return nil, ErrInvalidWorkflowHandler
	}
	return &WorkflowHandler{manager: manager, accounts: accounts}, nil
}

type workflowRequest struct {
	TranslationID   string `json:"translation_id"`
	LocaleID        string `json:"locale_id"`
	ExpectedVersion int64  `json:"expected_version"`
	Action          string `json:"action"`
}

type workflowResponse struct {
	ContentID          string `json:"content_id"`
	TranslationID      string `json:"translation_id"`
	ContentStatus      string `json:"content_status"`
	TranslationStatus  string `json:"translation_status"`
	ContentVersion     int64  `json:"content_version"`
	TranslationVersion int64  `json:"translation_version"`
}

func (handler *WorkflowHandler) Transition(ctx *gin.Context) {
	session, ok := platformmiddleware.SessionFromContext(ctx.Request.Context())
	if !ok || session.State != identitydomain.SessionStateAuthenticated || session.UserID == nil || strings.TrimSpace(*session.UserID) == "" {
		writeWorkflowProblem(ctx, http.StatusUnauthorized, "authentication_required", "Authentication is required.", nil)
		return
	}
	resolved, err := handler.accounts.Execute(ctx.Request.Context(), *session.UserID)
	if err != nil || resolved.Account.ID == "" {
		writeWorkflowProblem(ctx, http.StatusForbidden, "permission_denied", "Permission is required.", err)
		return
	}
	requestID := platformmiddleware.RequestIDFromContext(ctx.Request.Context())
	if strings.TrimSpace(requestID) == "" {
		writeWorkflowProblem(ctx, http.StatusBadRequest, "invalid_request", "A request ID is required for workflow mutations.", nil)
		return
	}
	if err := requireJSONContentType(ctx); err != nil {
		writeWorkflowProblem(ctx, http.StatusUnsupportedMediaType, "unsupported_media_type", "Workflow mutations require application/json.", err)
		return
	}
	var request workflowRequest
	if err := decodeStrictJSON(ctx, &request); err != nil {
		writeWorkflowProblem(ctx, http.StatusBadRequest, "invalid_request", "The workflow request is invalid.", err)
		return
	}
	detail, err := handler.manager.Transition(ctx.Request.Context(), workflowapp.TransitionInput{
		AccountID: resolved.Account.ID, ActorUserID: *session.UserID, RequestID: requestID,
		ContentID: contentIDParam(ctx), TranslationID: strings.TrimSpace(request.TranslationID),
		LocaleID: strings.TrimSpace(request.LocaleID), ExpectedVersion: request.ExpectedVersion,
		Action: strings.TrimSpace(request.Action),
	})
	if err != nil {
		writeWorkflowError(ctx, err)
		return
	}
	ctx.Header("Cache-Control", "no-store")
	ctx.JSON(http.StatusOK, workflowResponse{
		ContentID: detail.ContentID, TranslationID: detail.TranslationID,
		ContentStatus: string(detail.ContentStatus), TranslationStatus: string(detail.TranslationStatus),
		ContentVersion: detail.ContentVersion, TranslationVersion: detail.TranslationVersion,
	})
}

func writeWorkflowError(ctx *gin.Context, err error) {
	switch {
	case errors.Is(err, workflowapp.ErrWorkflowDenied):
		writeWorkflowProblem(ctx, http.StatusForbidden, "permission_denied", "Permission is required.", nil)
	case errors.Is(err, ports.ErrWorkflowConflict), errors.Is(err, publishingdomain.ErrInvalidWorkflowTransition):
		writeWorkflowProblem(ctx, http.StatusConflict, "content_version_conflict", "The workflow state changed or the transition is invalid.", nil)
	case errors.Is(err, workflowapp.ErrInvalidWorkflowCommand):
		writeWorkflowProblem(ctx, http.StatusUnprocessableEntity, "invalid_request", "The workflow request is invalid.", nil)
	case errors.Is(err, ports.ErrWorkflowNotFound):
		writeWorkflowProblem(ctx, http.StatusNotFound, "not_found", "The content was not found.", nil)
	default:
		writeWorkflowProblem(ctx, http.StatusInternalServerError, "internal_error", "The workflow transition could not be completed.", err)
	}
}

func writeWorkflowProblem(ctx *gin.Context, status int, code, message string, cause error) {
	response.WriteProblem(ctx, status, platformmiddleware.RequestIDFromContext(ctx.Request.Context()), response.NewProblem(code, message, nil, cause))
}

var _ workflowManager = (*workflowapp.Service)(nil)
