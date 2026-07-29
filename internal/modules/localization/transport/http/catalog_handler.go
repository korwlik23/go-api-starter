package http

import (
	"context"
	"errors"
	nethttp "net/http"

	"github.com/gin-gonic/gin"
	adminapi "github.com/korwlik23/go-api-starter/internal/generated/admin"
	catalogapp "github.com/korwlik23/go-api-starter/internal/modules/localization/application/catalog"
	"github.com/korwlik23/go-api-starter/internal/modules/localization/ports"
)

type catalogReader interface {
	Execute(context.Context, string, string) (catalogapp.Result, error)
}

type catalogUpdater interface {
	Execute(
		context.Context,
		string,
		catalogapp.UpdateCommand,
	) (int64, error)
}

type CatalogHandler struct {
	read   catalogReader
	update catalogUpdater
}

func NewCatalogReadHandler(
	read catalogReader,
) (*CatalogHandler, error) {
	if read == nil {
		return nil, ErrInvalidLocaleHandler
	}
	return &CatalogHandler{read: read}, nil
}

func NewCatalogHandler(
	read catalogReader,
	update catalogUpdater,
) (*CatalogHandler, error) {
	if read == nil || update == nil {
		return nil, ErrInvalidLocaleHandler
	}
	return &CatalogHandler{read: read, update: update}, nil
}

func (handler *CatalogHandler) Get(ctx *gin.Context) {
	result, err := handler.read.Execute(
		ctx.Request.Context(),
		ctx.Query("locale"),
		ctx.Query("category"),
	)
	if err != nil {
		status := nethttp.StatusBadRequest
		if errors.Is(err, ports.ErrCatalogNotFound) {
			status = nethttp.StatusNotFound
		}
		writeLocalizationProblem(ctx, status, err)
		return
	}
	ctx.Header("Cache-Control", "public, max-age=60")
	ctx.JSON(nethttp.StatusOK, adminapi.TranslationCatalogResponse{
		Locale: result.Locale, Category: result.Category,
		Version: int(result.Version), Entries: result.Entries,
	})
}

func (handler *CatalogHandler) Update(ctx *gin.Context) {
	userID, ok := authenticatedUserID(ctx)
	if !ok {
		writeLocalizationProblem(ctx, nethttp.StatusUnauthorized, nil)
		return
	}
	var request adminapi.TranslationCatalogUpdateRequest
	if err := ctx.ShouldBindJSON(&request); err != nil {
		writeLocalizationProblem(ctx, nethttp.StatusBadRequest, err)
		return
	}
	version, err := handler.update.Execute(
		ctx.Request.Context(),
		userID,
		catalogapp.UpdateCommand{
			Locale: request.Locale, Category: request.Category,
			Entries:         request.Entries,
			ExpectedVersion: int64(request.ExpectedVersion),
		},
	)
	if err != nil {
		status := nethttp.StatusBadRequest
		switch {
		case errors.Is(err, catalogapp.ErrCatalogUpdateDenied):
			status = nethttp.StatusForbidden
		case errors.Is(err, ports.ErrCatalogConflict):
			status = nethttp.StatusConflict
		case errors.Is(err, ports.ErrCatalogNotFound):
			status = nethttp.StatusNotFound
		}
		writeLocalizationProblem(ctx, status, err)
		return
	}
	ctx.JSON(nethttp.StatusOK, adminapi.TranslationCatalogUpdateResponse{
		Locale: request.Locale, Category: request.Category,
		Version: int(version),
	})
}
