package http

import (
	"context"
	"errors"
	nethttp "net/http"

	"github.com/gin-gonic/gin"
	adminapi "github.com/korwlik23/go-api-starter/internal/generated/admin"
	identitydomain "github.com/korwlik23/go-api-starter/internal/modules/identity/domain"
	localizationapp "github.com/korwlik23/go-api-starter/internal/modules/localization/application"
	localizationdomain "github.com/korwlik23/go-api-starter/internal/modules/localization/domain"
	"github.com/korwlik23/go-api-starter/internal/modules/localization/ports"
	platformmiddleware "github.com/korwlik23/go-api-starter/internal/platform/middleware"
)

type localeManager interface {
	Create(
		context.Context,
		string,
		localizationapp.CreateLocaleInput,
	) (localizationdomain.Locale, error)
	UpdateMetadata(
		context.Context,
		string,
		string,
		localizationapp.UpdateLocaleMetadataInput,
	) (int64, error)
	UpdateStatus(
		context.Context,
		string,
		string,
		localizationapp.UpdateLocaleStatusInput,
	) (int64, error)
	SetDefault(
		context.Context,
		string,
		string,
		int64,
	) (ports.DefaultLocaleResult, error)
}

type LocaleManagementHandler struct {
	manager localeManager
}

func NewLocaleManagementHandler(
	manager localeManager,
) (*LocaleManagementHandler, error) {
	if manager == nil {
		return nil, ErrInvalidLocaleHandler
	}
	return &LocaleManagementHandler{manager: manager}, nil
}

func (handler *LocaleManagementHandler) Create(ctx *gin.Context) {
	userID, ok := authenticatedUserID(ctx)
	if !ok {
		writeLocalizationProblem(ctx, nethttp.StatusUnauthorized, nil)
		return
	}
	var request adminapi.LocaleCreateRequest
	if err := ctx.ShouldBindJSON(&request); err != nil {
		writeLocalizationProblem(ctx, nethttp.StatusBadRequest, err)
		return
	}
	locale, err := handler.manager.Create(
		ctx.Request.Context(),
		userID,
		localizationapp.CreateLocaleInput{
			Tag: request.Code, Name: request.Name,
			Direction: string(request.Direction),
		},
	)
	if err != nil {
		writeLocalizationMutationError(ctx, err)
		return
	}
	ctx.JSON(nethttp.StatusCreated, localeResponse(locale))
}

func (handler *LocaleManagementHandler) UpdateMetadata(ctx *gin.Context) {
	userID, ok := authenticatedUserID(ctx)
	if !ok {
		writeLocalizationProblem(ctx, nethttp.StatusUnauthorized, nil)
		return
	}
	var request adminapi.LocaleUpdateRequest
	if err := ctx.ShouldBindJSON(&request); err != nil {
		writeLocalizationProblem(ctx, nethttp.StatusBadRequest, err)
		return
	}
	version, err := handler.manager.UpdateMetadata(
		ctx.Request.Context(),
		userID,
		ctx.Param("locale_id"),
		localizationapp.UpdateLocaleMetadataInput{
			Name: request.Name, Direction: string(request.Direction),
			ExpectedVersion: int64(request.ExpectedVersion),
		},
	)
	if err != nil {
		writeLocalizationMutationError(ctx, err)
		return
	}
	ctx.JSON(nethttp.StatusOK, adminapi.LocaleUpdateResponse{
		Id: ctx.Param("locale_id"), Name: request.Name,
		Direction: adminapi.LocaleUpdateResponseDirection(request.Direction),
		Version:   int(version),
	})
}

func (handler *LocaleManagementHandler) UpdateStatus(ctx *gin.Context) {
	userID, ok := authenticatedUserID(ctx)
	if !ok {
		writeLocalizationProblem(ctx, nethttp.StatusUnauthorized, nil)
		return
	}
	var request adminapi.LocaleStatusRequest
	if err := ctx.ShouldBindJSON(&request); err != nil {
		writeLocalizationProblem(ctx, nethttp.StatusBadRequest, err)
		return
	}
	version, err := handler.manager.UpdateStatus(
		ctx.Request.Context(),
		userID,
		ctx.Param("locale_id"),
		localizationapp.UpdateLocaleStatusInput{
			Enabled: request.Enabled, UserSelectable: request.Selectable,
			ExpectedVersion: int64(request.ExpectedVersion),
		},
	)
	if err != nil {
		writeLocalizationMutationError(ctx, err)
		return
	}
	ctx.JSON(nethttp.StatusOK, adminapi.LocaleStatusResponse{
		Id: ctx.Param("locale_id"), Enabled: request.Enabled,
		Selectable: request.Selectable, Version: int(version),
	})
}

func (handler *LocaleManagementHandler) SetDefault(ctx *gin.Context) {
	userID, ok := authenticatedUserID(ctx)
	if !ok {
		writeLocalizationProblem(ctx, nethttp.StatusUnauthorized, nil)
		return
	}
	var request adminapi.SetDefaultLocaleRequest
	if err := ctx.ShouldBindJSON(&request); err != nil {
		writeLocalizationProblem(ctx, nethttp.StatusBadRequest, err)
		return
	}
	result, err := handler.manager.SetDefault(
		ctx.Request.Context(),
		userID,
		ctx.Param("locale_id"),
		int64(request.ExpectedVersion),
	)
	if err != nil {
		writeLocalizationMutationError(ctx, err)
		return
	}
	ctx.JSON(nethttp.StatusOK, adminapi.DefaultLocaleUpdateResult{
		LocaleId:                ctx.Param("locale_id"),
		PreviousDefaultLocaleId: result.PreviousDefaultLocaleID,
		Version:                 int(result.Version),
	})
}

func authenticatedUserID(ctx *gin.Context) (string, bool) {
	session, ok := platformmiddleware.SessionFromContext(ctx.Request.Context())
	if !ok ||
		session.State != identitydomain.SessionStateAuthenticated ||
		session.UserID == nil {
		return "", false
	}
	return *session.UserID, true
}

func writeLocalizationMutationError(ctx *gin.Context, err error) {
	status := nethttp.StatusBadRequest
	switch {
	case errors.Is(err, localizationapp.ErrLocaleManagementDenied):
		status = nethttp.StatusForbidden
	case errors.Is(err, ports.ErrLocaleNotFound):
		status = nethttp.StatusNotFound
	case errors.Is(err, ports.ErrLocaleConflict):
		status = nethttp.StatusConflict
	}
	writeLocalizationProblem(ctx, status, err)
}
