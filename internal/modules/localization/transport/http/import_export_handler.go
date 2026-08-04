package http

import (
	"context"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	catalogapp "github.com/korwlik23/go-api-starter/internal/modules/localization/application/catalog"
	platformmiddleware "github.com/korwlik23/go-api-starter/internal/platform/middleware"
)

type catalogImporterExporter interface {
	Export(context.Context, string, string, string) (catalogapp.ExportResult, error)
	Import(context.Context, string, string, string, map[string]string, int64, string) (int64, error)
}
type ImportExportHandler struct{ service catalogImporterExporter }

func NewImportExportHandler(service catalogImporterExporter) (*ImportExportHandler, error) {
	if service == nil {
		return nil, errors.New("invalid catalog import/export handler")
	}
	return &ImportExportHandler{service: service}, nil
}
func (handler *ImportExportHandler) Export(ctx *gin.Context) {
	actor, ok := authenticatedUserID(ctx)
	if !ok {
		writeLocalizationProblem(ctx, http.StatusUnauthorized, nil)
		return
	}
	result, err := handler.service.Export(ctx.Request.Context(), actor, ctx.Query("locale"), ctx.Query("category"))
	if err != nil {
		writeLocalizationMutationError(ctx, err)
		return
	}
	ctx.Header("Content-Disposition", "attachment; filename=localization-"+result.Locale+"-"+result.Category+".json")
	ctx.JSON(http.StatusOK, result)
}

type importRequest struct {
	Locale          string            `json:"locale"`
	Category        string            `json:"category"`
	ExpectedVersion int64             `json:"expected_version"`
	Entries         map[string]string `json:"entries"`
}

func (handler *ImportExportHandler) Import(ctx *gin.Context) {
	actor, ok := authenticatedUserID(ctx)
	if !ok {
		writeLocalizationProblem(ctx, http.StatusUnauthorized, nil)
		return
	}
	var request importRequest
	if err := ctx.ShouldBindJSON(&request); err != nil {
		writeLocalizationProblem(ctx, http.StatusBadRequest, err)
		return
	}
	version, err := handler.service.Import(ctx.Request.Context(), actor, request.Locale, request.Category, request.Entries, request.ExpectedVersion, platformmiddleware.RequestIDFromContext(ctx.Request.Context()))
	if err != nil {
		writeLocalizationMutationError(ctx, err)
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"locale": request.Locale, "category": request.Category, "version": version})
}
