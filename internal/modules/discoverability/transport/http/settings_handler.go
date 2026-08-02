package http

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	settingsapp "github.com/korwlik23/go-api-starter/internal/modules/discoverability/application/settings"
	discoverabilitydomain "github.com/korwlik23/go-api-starter/internal/modules/discoverability/domain"
	platformmiddleware "github.com/korwlik23/go-api-starter/internal/platform/middleware"
)

var ErrInvalidSettingsHandler = errors.New("invalid discoverability settings handler dependencies")

type settingsManager interface {
	Get(context.Context, string, string) (discoverabilitydomain.SEODefaults, error)
	Update(context.Context, settingsapp.UpdateInput) (discoverabilitydomain.SEODefaults, error)
}
type SettingsHandler struct {
	manager  settingsManager
	accounts accountResolver
}

func NewSettingsHandler(manager settingsManager, accounts accountResolver) (*SettingsHandler, error) {
	if manager == nil || accounts == nil {
		return nil, ErrInvalidSettingsHandler
	}
	return &SettingsHandler{manager: manager, accounts: accounts}, nil
}

type seoUpdateRequest struct {
	LocaleID        string `json:"locale_id"`
	Title           string `json:"title"`
	Description     string `json:"description"`
	CanonicalBase   string `json:"canonical_base"`
	Robots          string `json:"robots"`
	ExpectedVersion int64  `json:"expected_version"`
}
type seoResponse struct {
	ID            string `json:"id"`
	AccountID     string `json:"account_id"`
	LocaleID      string `json:"locale_id"`
	Title         string `json:"title"`
	Description   string `json:"description"`
	CanonicalBase string `json:"canonical_base"`
	Robots        string `json:"robots"`
	Version       int64  `json:"version"`
	CreatedAt     string `json:"created_at"`
	UpdatedAt     string `json:"updated_at"`
}

func (handler *SettingsHandler) Get(ctx *gin.Context) {
	_, accountID, ok := principal(ctx, handler.accounts)
	if !ok {
		return
	}
	defaults, err := handler.manager.Get(ctx.Request.Context(), accountID, strings.TrimSpace(ctx.Query("locale_id")))
	if err != nil {
		writeDiscoverabilityError(ctx, err)
		return
	}
	ctx.Header("Cache-Control", "no-store")
	ctx.JSON(http.StatusOK, toSEOResponse(defaults))
}
func (handler *SettingsHandler) Update(ctx *gin.Context) {
	userID, accountID, ok := principal(ctx, handler.accounts)
	if !ok || !prepareJSON(ctx) {
		return
	}
	var request seoUpdateRequest
	if !decodeJSON(ctx, &request) {
		return
	}
	defaults, err := handler.manager.Update(ctx.Request.Context(), settingsapp.UpdateInput{AccountID: accountID, ActorUserID: userID, RequestID: platformmiddleware.RequestIDFromContext(ctx.Request.Context()), LocaleID: request.LocaleID, Title: request.Title, Description: request.Description, CanonicalBase: request.CanonicalBase, Robots: request.Robots, ExpectedVersion: request.ExpectedVersion})
	if err != nil {
		writeDiscoverabilityError(ctx, err)
		return
	}
	ctx.Header("Cache-Control", "no-store")
	ctx.JSON(http.StatusOK, toSEOResponse(defaults))
}
func toSEOResponse(defaults discoverabilitydomain.SEODefaults) seoResponse {
	return seoResponse{ID: defaults.ID, AccountID: defaults.AccountID, LocaleID: defaults.LocaleID, Title: defaults.Title, Description: defaults.Description, CanonicalBase: defaults.CanonicalBase, Robots: defaults.Robots, Version: defaults.Version, CreatedAt: defaults.CreatedAt.UTC().Format(time.RFC3339Nano), UpdatedAt: defaults.UpdatedAt.UTC().Format(time.RFC3339Nano)}
}
