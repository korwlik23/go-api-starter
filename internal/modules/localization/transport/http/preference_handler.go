package http

import (
	"context"
	"errors"
	nethttp "net/http"

	"github.com/gin-gonic/gin"
	adminapi "github.com/korwlik23/go-api-starter/internal/generated/admin"
	preferenceapp "github.com/korwlik23/go-api-starter/internal/modules/localization/application/preferences"
	localizationdomain "github.com/korwlik23/go-api-starter/internal/modules/localization/domain"
	"github.com/korwlik23/go-api-starter/internal/modules/localization/ports"
)

type preferenceReader interface {
	Execute(context.Context, string) (localizationdomain.LocalePreference, error)
}

type preferenceUpdater interface {
	Execute(
		context.Context,
		string,
		preferenceapp.UpdateCommand,
	) (localizationdomain.LocalePreference, error)
}

type PreferenceHandler struct {
	read   preferenceReader
	update preferenceUpdater
}

func NewPreferenceHandler(
	read preferenceReader,
	update preferenceUpdater,
) (*PreferenceHandler, error) {
	if read == nil || update == nil {
		return nil, ErrInvalidLocaleHandler
	}
	return &PreferenceHandler{read: read, update: update}, nil
}

func (handler *PreferenceHandler) Get(ctx *gin.Context) {
	userID, ok := authenticatedUserID(ctx)
	if !ok {
		writeLocalizationProblem(ctx, nethttp.StatusUnauthorized, nil)
		return
	}
	preference, err := handler.read.Execute(ctx.Request.Context(), userID)
	if err != nil {
		writeLocalizationProblem(ctx, nethttp.StatusInternalServerError, err)
		return
	}
	writePreference(ctx, preference)
}

func (handler *PreferenceHandler) Update(ctx *gin.Context) {
	userID, ok := authenticatedUserID(ctx)
	if !ok {
		writeLocalizationProblem(ctx, nethttp.StatusUnauthorized, nil)
		return
	}
	var request adminapi.LocalePreferenceRequest
	if err := ctx.ShouldBindJSON(&request); err != nil {
		writeLocalizationProblem(ctx, nethttp.StatusBadRequest, err)
		return
	}
	preference, err := handler.update.Execute(
		ctx.Request.Context(),
		userID,
		preferenceapp.UpdateCommand{
			AllLanguages:    request.AllLanguages,
			LocaleIDs:       request.LocaleIds,
			ExpectedVersion: int64(request.ExpectedVersion),
		},
	)
	if err != nil {
		status := nethttp.StatusBadRequest
		if errors.Is(err, ports.ErrLocalePreferenceConflict) {
			status = nethttp.StatusConflict
		}
		writeLocalizationProblem(ctx, status, err)
		return
	}
	writePreference(ctx, preference)
}

func writePreference(
	ctx *gin.Context,
	preference localizationdomain.LocalePreference,
) {
	ctx.Header("Cache-Control", "private, no-store")
	ctx.JSON(nethttp.StatusOK, adminapi.LocalePreferenceResponse{
		AllLanguages: preference.AllLanguages,
		LocaleIds:    preference.LocaleIDs,
		Version:      int(preference.Version),
	})
}
