package http

import (
	"context"
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	identitydomain "github.com/korwlik23/go-api-starter/internal/modules/identity/domain"
	settingsapp "github.com/korwlik23/go-api-starter/internal/modules/settings/application"
	settingsdomain "github.com/korwlik23/go-api-starter/internal/modules/settings/domain"
	"github.com/korwlik23/go-api-starter/internal/platform/middleware"
	"github.com/korwlik23/go-api-starter/internal/platform/response"
)

type service interface {
	List(context.Context, string) ([]settingsdomain.Setting, error)
	Update(context.Context, string, string, string, string, int64) (settingsdomain.Setting, error)
}

type Handler struct{ service service }

func NewHandler(service service) (*Handler, error) {
	if service == nil {
		return nil, errors.New("settings: invalid handler")
	}
	return &Handler{service: service}, nil
}

type updateRequest struct {
	Value           string `json:"value" validate:"max=16384"`
	ExpectedVersion int64  `json:"expected_version" validate:"gte=0"`
}

func (handler *Handler) List(ctx *gin.Context) {
	userID, ok := authenticatedUserID(ctx)
	if !ok {
		writeProblem(ctx, http.StatusUnauthorized, nil)
		return
	}
	items, err := handler.service.List(ctx.Request.Context(), userID)
	if err != nil {
		writeServiceError(ctx, err)
		return
	}
	ctx.Header("Cache-Control", "no-store")
	ctx.JSON(http.StatusOK, gin.H{"items": items})
}

func (handler *Handler) Update(ctx *gin.Context) {
	userID, ok := authenticatedUserID(ctx)
	if !ok {
		writeProblem(ctx, http.StatusUnauthorized, nil)
		return
	}
	var request updateRequest
	if err := ctx.ShouldBindJSON(&request); err != nil || validator.New().Struct(request) != nil {
		writeProblem(ctx, http.StatusBadRequest, err)
		return
	}
	item, err := handler.service.Update(ctx.Request.Context(), userID, ctx.Param("key"), request.Value, middleware.RequestIDFromContext(ctx.Request.Context()), request.ExpectedVersion)
	if err != nil {
		writeServiceError(ctx, err)
		return
	}
	ctx.JSON(http.StatusOK, item)
}

func authenticatedUserID(ctx *gin.Context) (string, bool) {
	session, ok := middleware.SessionFromContext(ctx.Request.Context())
	if !ok || session.State != identitydomain.SessionStateAuthenticated || session.UserID == nil || *session.UserID == "" {
		return "", false
	}
	return *session.UserID, true
}

func writeServiceError(ctx *gin.Context, err error) {
	status := http.StatusBadRequest
	if errors.Is(err, settingsapp.ErrDenied) {
		status = http.StatusForbidden
	}
	if errors.Is(err, settingsdomain.ErrVersionConflict) {
		status = http.StatusConflict
	}
	if errors.Is(err, settingsdomain.ErrSecretSetting) {
		status = http.StatusUnprocessableEntity
	}
	writeProblem(ctx, status, err)
}

func writeProblem(ctx *gin.Context, status int, cause error) {
	response.WriteProblem(ctx, status, middleware.RequestIDFromContext(ctx.Request.Context()), response.NewProblem("settings_request_failed", "The settings request could not be completed.", nil, cause))
}

var _ = strconv.IntSize
