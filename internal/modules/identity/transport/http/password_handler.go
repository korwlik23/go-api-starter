package http

import (
	"context"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	identityapp "github.com/korwlik23/go-api-starter/internal/modules/identity/application"
	"github.com/korwlik23/go-api-starter/internal/modules/identity/ports"
)

var ErrInvalidPasswordHandler = errors.New("invalid password handler")

type passwordChanger interface {
	Execute(context.Context, string, string, string) error
}

type PasswordHandler struct {
	service     passwordChanger
	clearCookie SessionCookieClearer
}

func NewPasswordHandler(service passwordChanger, clearCookie SessionCookieClearer) (*PasswordHandler, error) {
	if service == nil || clearCookie == nil {
		return nil, ErrInvalidPasswordHandler
	}
	return &PasswordHandler{service: service, clearCookie: clearCookie}, nil
}

type passwordChangeRequest struct {
	CurrentPassword string `json:"current_password"`
	NewPassword     string `json:"new_password"`
}

func (handler *PasswordHandler) Change(ctx *gin.Context) {
	userID, ok := authenticatedProfileUser(ctx)
	if !ok {
		return
	}
	var request passwordChangeRequest
	if err := decodeStrictJSON(ctx, &request); err != nil || request.CurrentPassword == "" || request.NewPassword == "" {
		writeProfileProblem(ctx, http.StatusBadRequest, err)
		return
	}
	if err := handler.service.Execute(ctx.Request.Context(), userID, request.CurrentPassword, request.NewPassword); err != nil {
		status := http.StatusInternalServerError
		cause := err
		if errors.Is(err, identityapp.ErrInvalidPasswordChange) || errors.Is(err, identityapp.ErrCurrentPasswordInvalid) || errors.Is(err, ports.ErrPasswordNotFound) {
			status = http.StatusUnprocessableEntity
			cause = nil
		}
		writeProfileProblem(ctx, status, cause)
		return
	}
	handler.clearCookie(ctx)
	ctx.Header("Cache-Control", "no-store")
	ctx.Status(http.StatusNoContent)
}
