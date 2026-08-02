package http

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	mediaapp "github.com/korwlik23/go-api-starter/internal/modules/media/application"
	mediadomain "github.com/korwlik23/go-api-starter/internal/modules/media/domain"
	mediaports "github.com/korwlik23/go-api-starter/internal/modules/media/ports"
	platformmiddleware "github.com/korwlik23/go-api-starter/internal/platform/middleware"
)

var ErrInvalidAssetHandler = errors.New("invalid media asset handler dependencies")

type assetManager interface {
	ListAssets(context.Context, mediaapp.ListAssetsInput) (mediaports.AssetListPage, error)
	DeleteAsset(context.Context, mediaapp.DeleteAssetInput) error
}

type AssetHandler struct {
	manager  assetManager
	accounts mediaAccountResolver
}

func NewAssetHandler(manager assetManager, accounts mediaAccountResolver) (*AssetHandler, error) {
	if manager == nil || accounts == nil {
		return nil, ErrInvalidAssetHandler
	}
	return &AssetHandler{manager: manager, accounts: accounts}, nil
}

type deleteAssetRequest struct {
	ExpectedVersion int64 `json:"expected_version"`
}

type assetResponse struct {
	ID           string `json:"id"`
	AccountID    string `json:"account_id"`
	OriginalName string `json:"original_name"`
	MIMEType     string `json:"mime_type"`
	ByteSize     int64  `json:"byte_size"`
	SHA256       string `json:"sha256"`
	Width        int64  `json:"width,omitempty"`
	Height       int64  `json:"height,omitempty"`
	Version      int64  `json:"version"`
	CreatedAt    string `json:"created_at"`
	UpdatedAt    string `json:"updated_at"`
}

type assetListResponse struct {
	Items []assetResponse      `json:"items"`
	Next  *assetCursorResponse `json:"next,omitempty"`
}

type assetCursorResponse struct {
	UpdatedAt string `json:"updated_at"`
	ID        string `json:"id"`
}

func (handler *AssetHandler) List(ctx *gin.Context) {
	_, accountID, ok := mediaPrincipal(ctx, handler.accounts)
	if !ok {
		return
	}
	limit := 25
	if value := strings.TrimSpace(ctx.Query("limit")); value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil || parsed < 1 || parsed > 100 {
			writeMediaProblem(ctx, http.StatusBadRequest, "invalid_request", "The asset list request is invalid.", nil)
			return
		}
		limit = parsed
	}
	var cursor *mediaports.AssetCursor
	if rawUpdated := strings.TrimSpace(ctx.Query("after_updated_at")); rawUpdated != "" || strings.TrimSpace(ctx.Query("after_id")) != "" {
		updatedAt, err := time.Parse(time.RFC3339Nano, rawUpdated)
		if err != nil || strings.TrimSpace(ctx.Query("after_id")) == "" {
			writeMediaProblem(ctx, http.StatusBadRequest, "invalid_request", "The asset cursor is invalid.", nil)
			return
		}
		cursor = &mediaports.AssetCursor{UpdatedAt: updatedAt, ID: strings.TrimSpace(ctx.Query("after_id"))}
	}
	page, err := handler.manager.ListAssets(ctx.Request.Context(), mediaapp.ListAssetsInput{AccountID: accountID, After: cursor, Limit: limit})
	if err != nil {
		writeMediaError(ctx, err)
		return
	}
	items := make([]assetResponse, 0, len(page.Items))
	for _, asset := range page.Items {
		items = append(items, assetResponseFromDomain(asset))
	}
	result := assetListResponse{Items: items}
	if page.Next != nil {
		result.Next = &assetCursorResponse{UpdatedAt: page.Next.UpdatedAt.UTC().Format(time.RFC3339Nano), ID: page.Next.ID}
	}
	ctx.Header("Cache-Control", "no-store")
	ctx.JSON(http.StatusOK, result)
}

func (handler *AssetHandler) Delete(ctx *gin.Context) {
	userID, accountID, ok := mediaPrincipal(ctx, handler.accounts)
	if !ok || !requireMediaJSON(ctx) {
		return
	}
	var request deleteAssetRequest
	if err := decodeMediaJSON(ctx, &request); err != nil {
		writeMediaProblem(ctx, http.StatusBadRequest, "invalid_request", "The delete request is invalid.", nil)
		return
	}
	if err := handler.manager.DeleteAsset(ctx.Request.Context(), mediaapp.DeleteAssetInput{AccountID: accountID, ActorUserID: userID, RequestID: platformmiddleware.RequestIDFromContext(ctx.Request.Context()), AssetID: strings.TrimSpace(ctx.Param("asset_id")), ExpectedVersion: request.ExpectedVersion}); err != nil {
		writeMediaError(ctx, err)
		return
	}
	ctx.Status(http.StatusNoContent)
}

func assetResponseFromDomain(asset mediadomain.Asset) assetResponse {
	return assetResponse{ID: asset.ID, AccountID: asset.AccountID, OriginalName: asset.OriginalName, MIMEType: asset.MIMEType, ByteSize: asset.ByteSize, SHA256: asset.SHA256, Width: asset.Width, Height: asset.Height, Version: asset.Version, CreatedAt: asset.CreatedAt.UTC().Format(time.RFC3339Nano), UpdatedAt: asset.UpdatedAt.UTC().Format(time.RFC3339Nano)}
}

var _ assetManager = (*mediaapp.Service)(nil)
