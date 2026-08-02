package http

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	accountports "github.com/korwlik23/go-api-starter/internal/modules/accounts/ports"
	redirectsapp "github.com/korwlik23/go-api-starter/internal/modules/discoverability/application/redirects"
	settingsapp "github.com/korwlik23/go-api-starter/internal/modules/discoverability/application/settings"
	discoverabilitydomain "github.com/korwlik23/go-api-starter/internal/modules/discoverability/domain"
	"github.com/korwlik23/go-api-starter/internal/modules/discoverability/ports"
	identitydomain "github.com/korwlik23/go-api-starter/internal/modules/identity/domain"
	platformmiddleware "github.com/korwlik23/go-api-starter/internal/platform/middleware"
	"github.com/korwlik23/go-api-starter/internal/platform/response"
)

type accountResolver interface {
	Execute(context.Context, string) (accountports.ResolvedAccount, error)
}

func principal(ctx *gin.Context, accounts accountResolver) (string, string, bool) {
	session, ok := platformmiddleware.SessionFromContext(ctx.Request.Context())
	if !ok || session.State != identitydomain.SessionStateAuthenticated || session.UserID == nil || strings.TrimSpace(*session.UserID) == "" {
		writeProblem(ctx, http.StatusUnauthorized, "authentication_required", "Authentication is required.", nil)
		return "", "", false
	}
	resolved, err := accounts.Execute(ctx.Request.Context(), *session.UserID)
	if err != nil || strings.TrimSpace(resolved.Account.ID) == "" {
		writeProblem(ctx, http.StatusForbidden, "permission_denied", "Permission is required.", nil)
		return "", "", false
	}
	return *session.UserID, resolved.Account.ID, true
}

func prepareJSON(ctx *gin.Context) bool {
	if strings.TrimSpace(platformmiddleware.RequestIDFromContext(ctx.Request.Context())) == "" {
		writeProblem(ctx, http.StatusBadRequest, "invalid_request", "A request ID is required for discoverability mutations.", nil)
		return false
	}
	value, _, err := mime.ParseMediaType(ctx.GetHeader("Content-Type"))
	if err != nil || !strings.EqualFold(value, "application/json") {
		writeProblem(ctx, http.StatusUnsupportedMediaType, "unsupported_media_type", "This mutation requires application/json.", nil)
		return false
	}
	return true
}

func decodeJSON(ctx *gin.Context, destination any) bool {
	const maxBody = 512 * 1024
	raw, err := io.ReadAll(io.LimitReader(ctx.Request.Body, maxBody+1))
	if err != nil || len(raw) > maxBody {
		writeProblem(ctx, http.StatusBadRequest, "invalid_request", "The discoverability request is invalid.", nil)
		return false
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		writeProblem(ctx, http.StatusBadRequest, "invalid_request", "The discoverability request is invalid.", nil)
		return false
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		writeProblem(ctx, http.StatusBadRequest, "invalid_request", "The discoverability request is invalid.", nil)
		return false
	}
	return true
}

func writeProblem(ctx *gin.Context, status int, code, message string, cause error) {
	response.WriteProblem(ctx, status, platformmiddleware.RequestIDFromContext(ctx.Request.Context()), response.NewProblem(code, message, nil, cause))
}

func writeDiscoverabilityError(ctx *gin.Context, err error) {
	switch {
	case errors.Is(err, ports.ErrRedirectNotFound), errors.Is(err, ports.ErrSEODefaultsNotFound):
		writeProblem(ctx, http.StatusNotFound, "not_found", "The discoverability resource was not found.", nil)
	case errors.Is(err, ports.ErrRedirectConflict), errors.Is(err, ports.ErrSEODefaultsConflict):
		writeProblem(ctx, http.StatusConflict, "conflict", "The discoverability resource changed; reload before retrying.", nil)
	case errors.Is(err, redirectsapp.ErrInvalidRedirectMutation), errors.Is(err, settingsapp.ErrInvalidSEOSettings), errors.Is(err, discoverabilitydomain.ErrInvalidRedirect), errors.Is(err, discoverabilitydomain.ErrRedirectLoop), errors.Is(err, discoverabilitydomain.ErrInvalidSEODefaults), errors.Is(err, ports.ErrInvalidRedirectQuery):
		writeProblem(ctx, http.StatusUnprocessableEntity, "invalid_request", "The discoverability request is invalid.", nil)
	default:
		writeProblem(ctx, http.StatusInternalServerError, "internal_error", "The discoverability request could not be completed.", nil)
	}
}
