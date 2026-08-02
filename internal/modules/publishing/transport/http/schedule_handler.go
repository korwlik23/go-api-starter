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
	schedulesapp "github.com/korwlik23/go-api-starter/internal/modules/publishing/application/schedules"
	publishingdomain "github.com/korwlik23/go-api-starter/internal/modules/publishing/domain"
	"github.com/korwlik23/go-api-starter/internal/modules/publishing/ports"
	platformmiddleware "github.com/korwlik23/go-api-starter/internal/platform/middleware"
	"github.com/korwlik23/go-api-starter/internal/platform/response"
)

var ErrInvalidScheduleHandler = errors.New("invalid schedule handler dependencies")

type scheduleManager interface {
	Create(context.Context, schedulesapp.CreateInput) (publishingdomain.PublicationSchedule, error)
	List(context.Context, schedulesapp.ListInput) (ports.SchedulePage, error)
	Cancel(context.Context, schedulesapp.CancelInput) error
}

type ScheduleHandler struct {
	manager  scheduleManager
	accounts accountResolver
}

func NewScheduleHandler(manager scheduleManager, accounts accountResolver) (*ScheduleHandler, error) {
	if manager == nil || accounts == nil {
		return nil, ErrInvalidScheduleHandler
	}
	return &ScheduleHandler{manager: manager, accounts: accounts}, nil
}

type scheduleRequest struct {
	TranslationID   string     `json:"translation_id"`
	PublishAt       time.Time  `json:"publish_at"`
	UnpublishAt     *time.Time `json:"unpublish_at"`
	ExpectedVersion int64      `json:"expected_version"`
	IdempotencyKey  string     `json:"idempotency_key"`
}

type scheduleResponse struct {
	ID              string                         `json:"id"`
	AccountID       string                         `json:"account_id"`
	TranslationID   string                         `json:"translation_id"`
	PublishAt       time.Time                      `json:"publish_at"`
	UnpublishAt     *time.Time                     `json:"unpublish_at,omitempty"`
	ExpectedVersion int64                          `json:"expected_version"`
	IdempotencyKey  string                         `json:"idempotency_key"`
	State           publishingdomain.ScheduleState `json:"state"`
	Attempts        int                            `json:"attempts"`
}

type scheduleListResponse struct {
	Items []scheduleResponse `json:"items"`
}

func (handler *ScheduleHandler) List(ctx *gin.Context) {
	userID, accountID, ok := schedulePrincipal(ctx, handler.accounts)
	if !ok {
		return
	}
	_ = userID
	limit := 25
	if raw := strings.TrimSpace(ctx.Query("limit")); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 1 || parsed > 100 {
			writeScheduleProblem(ctx, http.StatusBadRequest, "invalid_request", "The schedule list request is invalid.", err)
			return
		}
		limit = parsed
	}
	page, err := handler.manager.List(ctx.Request.Context(), schedulesapp.ListInput{AccountID: accountID, Limit: limit})
	if err != nil {
		writeScheduleError(ctx, err)
		return
	}
	items := make([]scheduleResponse, 0, len(page.Items))
	for _, item := range page.Items {
		items = append(items, scheduleResponseFromDomain(item))
	}
	ctx.Header("Cache-Control", "no-store")
	ctx.JSON(http.StatusOK, scheduleListResponse{Items: items})
}

func (handler *ScheduleHandler) Create(ctx *gin.Context) {
	userID, accountID, ok := schedulePrincipal(ctx, handler.accounts)
	if !ok {
		return
	}
	if !scheduleMutationReady(ctx) {
		return
	}
	var request scheduleRequest
	if err := decodeStrictJSON(ctx, &request); err != nil {
		writeScheduleProblem(ctx, http.StatusBadRequest, "invalid_request", "The schedule request is invalid.", err)
		return
	}
	schedule, err := handler.manager.Create(ctx.Request.Context(), schedulesapp.CreateInput{
		AccountID: accountID, ActorUserID: userID, RequestID: platformmiddleware.RequestIDFromContext(ctx.Request.Context()),
		TranslationID: strings.TrimSpace(request.TranslationID), PublishAt: request.PublishAt, UnpublishAt: request.UnpublishAt,
		ExpectedVersion: request.ExpectedVersion, IdempotencyKey: strings.TrimSpace(request.IdempotencyKey),
	})
	if err != nil {
		writeScheduleError(ctx, err)
		return
	}
	ctx.Header("Cache-Control", "no-store")
	ctx.JSON(http.StatusCreated, scheduleResponseFromDomain(schedule))
}

func (handler *ScheduleHandler) Cancel(ctx *gin.Context) {
	userID, accountID, ok := schedulePrincipal(ctx, handler.accounts)
	if !ok {
		return
	}
	if !scheduleMutationReady(ctx) {
		return
	}
	if err := handler.manager.Cancel(ctx.Request.Context(), schedulesapp.CancelInput{
		AccountID: accountID, ActorUserID: userID, RequestID: platformmiddleware.RequestIDFromContext(ctx.Request.Context()), ScheduleID: strings.TrimSpace(ctx.Param("schedule_id")),
	}); err != nil {
		writeScheduleError(ctx, err)
		return
	}
	ctx.Status(http.StatusNoContent)
}

func schedulePrincipal(ctx *gin.Context, accounts accountResolver) (string, string, bool) {
	session, ok := platformmiddleware.SessionFromContext(ctx.Request.Context())
	if !ok || session.State != identitydomain.SessionStateAuthenticated || session.UserID == nil || strings.TrimSpace(*session.UserID) == "" {
		writeScheduleProblem(ctx, http.StatusUnauthorized, "authentication_required", "Authentication is required.", nil)
		return "", "", false
	}
	resolved, err := accounts.Execute(ctx.Request.Context(), *session.UserID)
	if err != nil || resolved.Account.ID == "" {
		writeScheduleProblem(ctx, http.StatusForbidden, "permission_denied", "Permission is required.", err)
		return "", "", false
	}
	return *session.UserID, resolved.Account.ID, true
}

func scheduleMutationReady(ctx *gin.Context) bool {
	if strings.TrimSpace(platformmiddleware.RequestIDFromContext(ctx.Request.Context())) == "" {
		writeScheduleProblem(ctx, http.StatusBadRequest, "invalid_request", "A request ID is required for schedule mutations.", nil)
		return false
	}
	if err := requireJSONContentType(ctx); err != nil {
		writeScheduleProblem(ctx, http.StatusUnsupportedMediaType, "unsupported_media_type", "Schedule mutations require application/json.", err)
		return false
	}
	return true
}

func scheduleResponseFromDomain(schedule publishingdomain.PublicationSchedule) scheduleResponse {
	return scheduleResponse{ID: schedule.ID, AccountID: schedule.AccountID, TranslationID: schedule.TranslationID, PublishAt: schedule.PublishAt, UnpublishAt: schedule.UnpublishAt, ExpectedVersion: schedule.ExpectedVersion, IdempotencyKey: schedule.IdempotencyKey, State: schedule.State, Attempts: schedule.Attempts}
}

func writeScheduleError(ctx *gin.Context, err error) {
	switch {
	case errors.Is(err, ports.ErrScheduleConflict):
		writeScheduleProblem(ctx, http.StatusConflict, "conflict", "The schedule changed before the request completed.", nil)
	case errors.Is(err, ports.ErrScheduleNotFound):
		writeScheduleProblem(ctx, http.StatusNotFound, "not_found", "The schedule was not found.", nil)
	case errors.Is(err, schedulesapp.ErrInvalidScheduleCommand):
		writeScheduleProblem(ctx, http.StatusUnprocessableEntity, "invalid_request", "The schedule request is invalid.", nil)
	default:
		writeScheduleProblem(ctx, http.StatusInternalServerError, "internal_error", "The schedule request could not be completed.", err)
	}
}

func writeScheduleProblem(ctx *gin.Context, status int, code, message string, cause error) {
	response.WriteProblem(ctx, status, platformmiddleware.RequestIDFromContext(ctx.Request.Context()), response.NewProblem(code, message, nil, cause))
}

var _ scheduleManager = (*schedulesapp.Service)(nil)
