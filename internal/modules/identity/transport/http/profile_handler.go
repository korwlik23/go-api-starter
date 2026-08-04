package http

import (
	"context"
	"errors"
	"github.com/gin-gonic/gin"
	identityapp "github.com/korwlik23/go-api-starter/internal/modules/identity/application"
	identitydomain "github.com/korwlik23/go-api-starter/internal/modules/identity/domain"
	"github.com/korwlik23/go-api-starter/internal/modules/identity/ports"
	platformmiddleware "github.com/korwlik23/go-api-starter/internal/platform/middleware"
	"github.com/korwlik23/go-api-starter/internal/platform/response"
	"net/http"
)

type profileService interface {
	Get(context.Context, string) (identitydomain.Profile, error)
	Update(context.Context, string, string, string, string, int64) (identitydomain.Profile, error)
}
type ProfileHandler struct{ service profileService }

func NewProfileHandler(service profileService) (*ProfileHandler, error) {
	if service == nil {
		return nil, errors.New("invalid profile handler")
	}
	return &ProfileHandler{service: service}, nil
}

type profileUpdateRequest struct {
	DisplayName     string `json:"display_name"`
	Timezone        string `json:"timezone"`
	PreferredLocale string `json:"preferred_locale"`
	ExpectedVersion int64  `json:"expected_version"`
}

func (handler *ProfileHandler) Get(ctx *gin.Context) {
	user, ok := authenticatedProfileUser(ctx)
	if !ok {
		return
	}
	profile, err := handler.service.Get(ctx.Request.Context(), user)
	if err != nil {
		writeProfileError(ctx, err)
		return
	}
	ctx.Header("Cache-Control", "no-store")
	ctx.JSON(http.StatusOK, profile)
}
func (handler *ProfileHandler) Update(ctx *gin.Context) {
	user, ok := authenticatedProfileUser(ctx)
	if !ok {
		return
	}
	var request profileUpdateRequest
	if err := ctx.ShouldBindJSON(&request); err != nil {
		writeProfileProblem(ctx, http.StatusBadRequest, err)
		return
	}
	profile, err := handler.service.Update(ctx.Request.Context(), user, request.DisplayName, request.Timezone, request.PreferredLocale, request.ExpectedVersion)
	if err != nil {
		writeProfileError(ctx, err)
		return
	}
	ctx.Header("Cache-Control", "no-store")
	ctx.JSON(http.StatusOK, profile)
}
func authenticatedProfileUser(ctx *gin.Context) (string, bool) {
	session, ok := platformmiddleware.SessionFromContext(ctx.Request.Context())
	if !ok || session.State != identitydomain.SessionStateAuthenticated || session.UserID == nil || *session.UserID == "" {
		writeProfileProblem(ctx, http.StatusUnauthorized, nil)
		return "", false
	}
	return *session.UserID, true
}
func writeProfileError(ctx *gin.Context, err error) {
	status := http.StatusBadRequest
	if errors.Is(err, identityapp.ErrInvalidProfileService) {
		status = http.StatusBadRequest
	}
	if errors.Is(err, identitydomain.ErrInvalidProfile) {
		status = http.StatusUnprocessableEntity
	}
	if errors.Is(err, ports.ErrProfileNotFound) {
		status = http.StatusNotFound
	}
	if errors.Is(err, ports.ErrProfileConflict) {
		status = http.StatusConflict
	}
	writeProfileProblem(ctx, status, err)
}
func writeProfileProblem(ctx *gin.Context, status int, cause error) {
	response.WriteProblem(ctx, status, platformmiddleware.RequestIDFromContext(ctx.Request.Context()), response.NewProblem("identity_profile_request_failed", "The profile request could not be completed.", nil, cause))
}
