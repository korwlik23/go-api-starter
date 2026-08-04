package http

import (
	"context"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	identitydomain "github.com/korwlik23/go-api-starter/internal/modules/identity/domain"
	notificationsapp "github.com/korwlik23/go-api-starter/internal/modules/notifications/application"
	notificationsdomain "github.com/korwlik23/go-api-starter/internal/modules/notifications/domain"
	"github.com/korwlik23/go-api-starter/internal/platform/middleware"
	"github.com/korwlik23/go-api-starter/internal/platform/response"
)

type service interface {
	List(context.Context, string, int) ([]notificationsdomain.Notification, error)
	MarkRead(context.Context, string, string) error
	MarkAllRead(context.Context, string) error
}

type Handler struct{ service service }

func NewHandler(service service) (*Handler, error) {
	if service == nil {
		return nil, errors.New("notifications: invalid handler")
	}
	return &Handler{service: service}, nil
}

func (handler *Handler) List(ctx *gin.Context) {
	userID, ok := authenticatedUserID(ctx)
	if !ok {
		writeProblem(ctx, http.StatusUnauthorized, nil)
		return
	}
	items, err := handler.service.List(ctx.Request.Context(), userID, 50)
	if err != nil {
		writeServiceError(ctx, err)
		return
	}
	ctx.Header("Cache-Control", "no-store")
	ctx.JSON(http.StatusOK, gin.H{"items": items})
}

func (handler *Handler) MarkRead(ctx *gin.Context) {
	userID, ok := authenticatedUserID(ctx)
	if !ok {
		writeProblem(ctx, http.StatusUnauthorized, nil)
		return
	}
	if err := handler.service.MarkRead(ctx.Request.Context(), userID, ctx.Param("notification_id")); err != nil {
		writeServiceError(ctx, err)
		return
	}
	ctx.Status(http.StatusNoContent)
}

func (handler *Handler) MarkAllRead(ctx *gin.Context) {
	userID, ok := authenticatedUserID(ctx)
	if !ok {
		writeProblem(ctx, http.StatusUnauthorized, nil)
		return
	}
	if err := handler.service.MarkAllRead(ctx.Request.Context(), userID); err != nil {
		writeServiceError(ctx, err)
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

func writeServiceError(ctx *gin.Context, err error) {
	status := http.StatusBadRequest
	if errors.Is(err, notificationsapp.ErrDenied) {
		status = http.StatusForbidden
	}
	if errors.Is(err, notificationsdomain.ErrInvalidNotification) {
		status = http.StatusNotFound
	}
	writeProblem(ctx, status, err)
}

func writeProblem(ctx *gin.Context, status int, cause error) {
	response.WriteProblem(ctx, status, middleware.RequestIDFromContext(ctx.Request.Context()), response.NewProblem("notifications_request_failed", "The notifications request could not be completed.", nil, cause))
}
