package http

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	accountports "github.com/korwlik23/go-api-starter/internal/modules/accounts/ports"
	identitydomain "github.com/korwlik23/go-api-starter/internal/modules/identity/domain"
	mediaapp "github.com/korwlik23/go-api-starter/internal/modules/media/application"
	mediadomain "github.com/korwlik23/go-api-starter/internal/modules/media/domain"
	mediaports "github.com/korwlik23/go-api-starter/internal/modules/media/ports"
	platformmiddleware "github.com/korwlik23/go-api-starter/internal/platform/middleware"
	"github.com/korwlik23/go-api-starter/internal/platform/response"
	platformstorage "github.com/korwlik23/go-api-starter/internal/platform/storage"
)

type mediaAccountResolver interface {
	Execute(context.Context, string) (accountports.ResolvedAccount, error)
}

func mediaPrincipal(ctx *gin.Context, accounts mediaAccountResolver) (string, string, bool) {
	session, ok := platformmiddleware.SessionFromContext(ctx.Request.Context())
	if !ok || session.State != identitydomain.SessionStateAuthenticated || session.UserID == nil || strings.TrimSpace(*session.UserID) == "" {
		writeMediaProblem(ctx, http.StatusUnauthorized, "authentication_required", "Authentication is required.", nil)
		return "", "", false
	}
	resolved, err := accounts.Execute(ctx.Request.Context(), *session.UserID)
	if err != nil || strings.TrimSpace(resolved.Account.ID) == "" {
		writeMediaProblem(ctx, http.StatusForbidden, "permission_denied", "Permission is required.", nil)
		return "", "", false
	}
	return *session.UserID, resolved.Account.ID, true
}

func requireMediaJSON(ctx *gin.Context) bool {
	if strings.TrimSpace(platformmiddleware.RequestIDFromContext(ctx.Request.Context())) == "" {
		writeMediaProblem(ctx, http.StatusBadRequest, "invalid_request", "A request ID is required for media mutations.", nil)
		return false
	}
	value, _, err := mime.ParseMediaType(ctx.GetHeader("Content-Type"))
	if err != nil || !strings.EqualFold(value, "application/json") {
		writeMediaProblem(ctx, http.StatusUnsupportedMediaType, "unsupported_media_type", "This media mutation requires application/json.", nil)
		return false
	}
	return true
}

func decodeMediaJSON(ctx *gin.Context, destination any) error {
	decoder := json.NewDecoder(io.LimitReader(ctx.Request.Body, 64*1024))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		return err
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		if err == nil {
			return errors.New("multiple JSON values")
		}
		return err
	}
	return nil
}

func writeMediaProblem(ctx *gin.Context, status int, code, message string, cause error) {
	response.WriteProblem(ctx, status, platformmiddleware.RequestIDFromContext(ctx.Request.Context()), response.NewProblem(code, message, nil, cause))
}

func writeMediaError(ctx *gin.Context, err error) {
	switch {
	case errors.Is(err, mediaports.ErrAssetNotFound), errors.Is(err, mediaports.ErrUploadNotFound):
		writeMediaProblem(ctx, http.StatusNotFound, "not_found", "The media resource was not found.", nil)
	case errors.Is(err, mediaports.ErrAssetConflict), errors.Is(err, mediaports.ErrUploadConflict), errors.Is(err, mediaapp.ErrDuplicateFinalize), errors.Is(err, mediaapp.ErrUploadExpired):
		writeMediaProblem(ctx, http.StatusConflict, "conflict", "The media resource changed or is no longer available.", nil)
	case errors.Is(err, mediaapp.ErrInvalidUploadCommand), errors.Is(err, mediaapp.ErrInvalidAssetCommand), errors.Is(err, mediaapp.ErrInvalidUploadContent), errors.Is(err, mediadomain.ErrInvalidAsset), errors.Is(err, mediadomain.ErrInvalidUpload), errors.Is(err, mediaports.ErrInvalidAssetQuery):
		writeMediaProblem(ctx, http.StatusUnprocessableEntity, "invalid_request", "The media request is invalid.", nil)
	case errors.Is(err, platformstorage.ErrSizeMismatch):
		writeMediaProblem(ctx, http.StatusUnprocessableEntity, "invalid_size", "The uploaded size does not match the declared size.", nil)
	default:
		writeMediaProblem(ctx, http.StatusInternalServerError, "internal_error", "The media request could not be completed.", nil)
	}
}
