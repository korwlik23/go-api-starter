package http

import (
	"context"
	"errors"
	nethttp "net/http"

	"github.com/gin-gonic/gin"
	adminapi "github.com/korwlik23/go-api-starter/internal/generated/admin"
	localizationdomain "github.com/korwlik23/go-api-starter/internal/modules/localization/domain"
	"github.com/korwlik23/go-api-starter/internal/platform/middleware"
	"github.com/korwlik23/go-api-starter/internal/platform/response"
)

var ErrInvalidLocaleHandler = errors.New("invalid locale handler dependencies")

type localeLister interface {
	Execute(context.Context) ([]localizationdomain.Locale, error)
}

type LocaleHandler struct {
	list localeLister
}

func NewLocaleHandler(list localeLister) (*LocaleHandler, error) {
	if list == nil {
		return nil, ErrInvalidLocaleHandler
	}
	return &LocaleHandler{list: list}, nil
}

func (handler *LocaleHandler) List(ctx *gin.Context) {
	locales, err := handler.list.Execute(ctx.Request.Context())
	if err != nil {
		writeLocalizationProblem(ctx, nethttp.StatusInternalServerError, err)
		return
	}
	items := make([]adminapi.Locale, 0, len(locales))
	for _, locale := range locales {
		items = append(items, localeResponse(locale))
	}
	ctx.Header("Cache-Control", "public, max-age=60")
	ctx.JSON(nethttp.StatusOK, adminapi.LocaleListResponse{Items: items})
}

func localeResponse(locale localizationdomain.Locale) adminapi.Locale {
	version := int(locale.Version)
	return adminapi.Locale{
		Id: locale.ID, Code: locale.Tag, Name: locale.Name,
		Direction: adminapi.LocaleDirection(locale.Direction),
		Enabled:   locale.Enabled, Selectable: locale.UserSelectable,
		Default: locale.Default, Version: &version,
	}
}

func writeLocalizationProblem(
	ctx *gin.Context,
	status int,
	cause error,
) {
	response.WriteProblem(
		ctx,
		status,
		middleware.RequestIDFromContext(ctx.Request.Context()),
		response.NewProblem(
			"localization_request_failed",
			"The localization request could not be completed.",
			nil,
			cause,
		),
	)
}
