package http

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	identitydomain "github.com/korwlik23/go-api-starter/internal/modules/identity/domain"
	operationsapp "github.com/korwlik23/go-api-starter/internal/modules/operations/application"
	operationsdomain "github.com/korwlik23/go-api-starter/internal/modules/operations/domain"
	"github.com/korwlik23/go-api-starter/internal/platform/middleware"
	"github.com/korwlik23/go-api-starter/internal/platform/response"
)

type jobService interface {
	List(context.Context, string, int) ([]operationsdomain.Job, error)
	Enqueue(context.Context, string, string, []byte) (operationsdomain.Job, error)
	Retry(context.Context, string, string) error
}
type JobHandler struct{ service jobService }

func NewJobHandler(service jobService) (*JobHandler, error) {
	if service == nil {
		return nil, errors.New("operations: invalid job handler")
	}
	return &JobHandler{service: service}, nil
}

type enqueueRequest struct {
	Name    string          `json:"name"`
	Payload json.RawMessage `json:"payload"`
}

func (handler *JobHandler) List(ctx *gin.Context) {
	user, ok := authenticatedUserID(ctx)
	if !ok {
		writeJobProblem(ctx, http.StatusUnauthorized, nil)
		return
	}
	items, err := handler.service.List(ctx.Request.Context(), user, 50)
	if err != nil {
		writeJobError(ctx, err)
		return
	}
	ctx.Header("Cache-Control", "no-store")
	ctx.JSON(http.StatusOK, gin.H{"items": items})
}
func (handler *JobHandler) Enqueue(ctx *gin.Context) {
	user, ok := authenticatedUserID(ctx)
	if !ok {
		writeJobProblem(ctx, http.StatusUnauthorized, nil)
		return
	}
	var request enqueueRequest
	if err := ctx.ShouldBindJSON(&request); err != nil || request.Name == "" || len(request.Payload) == 0 {
		writeJobProblem(ctx, http.StatusBadRequest, err)
		return
	}
	job, err := handler.service.Enqueue(ctx.Request.Context(), user, request.Name, request.Payload)
	if err != nil {
		writeJobError(ctx, err)
		return
	}
	ctx.JSON(http.StatusCreated, job)
}
func (handler *JobHandler) Retry(ctx *gin.Context) {
	user, ok := authenticatedUserID(ctx)
	if !ok {
		writeJobProblem(ctx, http.StatusUnauthorized, nil)
		return
	}
	if err := handler.service.Retry(ctx.Request.Context(), user, ctx.Param("job_id")); err != nil {
		writeJobError(ctx, err)
		return
	}
	ctx.Status(http.StatusNoContent)
}
func authenticatedUserID(ctx *gin.Context) (string, bool) {
	session, ok := middleware.SessionFromContext(ctx.Request.Context())
	if !ok || session.State != identitydomain.SessionStateAuthenticated || session.UserID == nil || *session.UserID == "" {
		return "", false
	}
	return *session.UserID, true
}
func writeJobError(ctx *gin.Context, err error) {
	status := http.StatusBadRequest
	if errors.Is(err, operationsapp.ErrJobDenied) {
		status = http.StatusForbidden
	}
	if errors.Is(err, operationsdomain.ErrInvalidJobTransition) {
		status = http.StatusConflict
	}
	writeJobProblem(ctx, status, err)
}
func writeJobProblem(ctx *gin.Context, status int, cause error) {
	response.WriteProblem(ctx, status, middleware.RequestIDFromContext(ctx.Request.Context()), response.NewProblem("operations_job_request_failed", "The operation job request could not be completed.", nil, cause))
}
