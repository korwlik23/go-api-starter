package http

import (
	"context"
	"errors"
	"mime"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	mediaapp "github.com/korwlik23/go-api-starter/internal/modules/media/application"
	mediadomain "github.com/korwlik23/go-api-starter/internal/modules/media/domain"
	platformmiddleware "github.com/korwlik23/go-api-starter/internal/platform/middleware"
)

var ErrInvalidUploadHandler = errors.New("invalid media upload handler dependencies")

type uploadManager interface {
	StartUpload(context.Context, mediaapp.StartUploadInput) (mediadomain.Upload, error)
	FinalizeUpload(context.Context, mediaapp.FinalizeUploadInput) (mediadomain.Asset, error)
}

type UploadHandler struct {
	manager  uploadManager
	accounts mediaAccountResolver
}

func NewUploadHandler(manager uploadManager, accounts mediaAccountResolver) (*UploadHandler, error) {
	if manager == nil || accounts == nil {
		return nil, ErrInvalidUploadHandler
	}
	return &UploadHandler{manager: manager, accounts: accounts}, nil
}

type finalizeUploadRequest struct {
	ExpectedVersion int64 `json:"expected_version"`
}

type uploadResponse struct {
	ID             string                  `json:"id"`
	AccountID      string                  `json:"account_id"`
	OriginalName   string                  `json:"original_name"`
	MIMEType       string                  `json:"mime_type"`
	DeclaredSize   int64                   `json:"declared_size"`
	ExpectedSHA256 string                  `json:"expected_sha256"`
	State          mediadomain.UploadState `json:"state"`
	AssetID        string                  `json:"asset_id,omitempty"`
	Version        int64                   `json:"version"`
	ExpiresAt      string                  `json:"expires_at"`
}

func (handler *UploadHandler) Start(ctx *gin.Context) {
	userID, accountID, ok := mediaPrincipal(ctx, handler.accounts)
	if !ok {
		return
	}
	requestID := platformmiddleware.RequestIDFromContext(ctx.Request.Context())
	if strings.TrimSpace(requestID) == "" {
		writeMediaProblem(ctx, http.StatusBadRequest, "invalid_request", "A request ID is required for media mutations.", nil)
		return
	}
	mimeType, _, err := mime.ParseMediaType(ctx.GetHeader("Content-Type"))
	if err != nil || !mediadomain.AllowedMIMEType(mimeType) {
		writeMediaProblem(ctx, http.StatusUnsupportedMediaType, "unsupported_media_type", "The upload MIME type is not allowed.", nil)
		return
	}
	if ctx.Request.ContentLength < 1 {
		writeMediaProblem(ctx, http.StatusLengthRequired, "length_required", "Content-Length is required for media uploads.", nil)
		return
	}
	if ctx.Request.ContentLength > mediadomain.MaxAssetSize {
		writeMediaProblem(ctx, http.StatusRequestEntityTooLarge, "payload_too_large", "The media upload is too large.", nil)
		return
	}
	fileName := strings.TrimSpace(ctx.GetHeader("X-File-Name"))
	sha256 := strings.TrimSpace(ctx.GetHeader("X-Content-SHA256"))
	if fileName == "" || sha256 == "" {
		writeMediaProblem(ctx, http.StatusBadRequest, "invalid_request", "X-File-Name and X-Content-SHA256 are required.", nil)
		return
	}
	upload, err := handler.manager.StartUpload(ctx.Request.Context(), mediaapp.StartUploadInput{
		AccountID: accountID, ActorUserID: userID, RequestID: requestID,
		OriginalName: fileName, MIMEType: mimeType, DeclaredSize: ctx.Request.ContentLength,
		ExpectedSHA256: sha256, Body: ctx.Request.Body,
	})
	if err != nil {
		writeMediaError(ctx, err)
		return
	}
	ctx.Header("Cache-Control", "no-store")
	ctx.JSON(http.StatusCreated, uploadResponseFromDomain(upload))
}

func (handler *UploadHandler) Finalize(ctx *gin.Context) {
	userID, accountID, ok := mediaPrincipal(ctx, handler.accounts)
	if !ok || !requireMediaJSON(ctx) {
		return
	}
	var request finalizeUploadRequest
	if err := decodeMediaJSON(ctx, &request); err != nil {
		writeMediaProblem(ctx, http.StatusBadRequest, "invalid_request", "The finalize request is invalid.", nil)
		return
	}
	asset, err := handler.manager.FinalizeUpload(ctx.Request.Context(), mediaapp.FinalizeUploadInput{
		AccountID: accountID, ActorUserID: userID, RequestID: platformmiddleware.RequestIDFromContext(ctx.Request.Context()),
		UploadID: strings.TrimSpace(ctx.Param("upload_id")), ExpectedVersion: request.ExpectedVersion,
	})
	if err != nil {
		writeMediaError(ctx, err)
		return
	}
	ctx.Header("Cache-Control", "no-store")
	ctx.JSON(http.StatusOK, assetResponseFromDomain(asset))
}

func uploadResponseFromDomain(upload mediadomain.Upload) uploadResponse {
	return uploadResponse{ID: upload.ID, AccountID: upload.AccountID, OriginalName: upload.OriginalName, MIMEType: upload.MIMEType, DeclaredSize: upload.DeclaredSize, ExpectedSHA256: upload.ExpectedSHA256, State: upload.State, AssetID: upload.AssetID, Version: upload.Version, ExpiresAt: upload.ExpiresAt.UTC().Format("2006-01-02T15:04:05.999999Z07:00")}
}

var _ uploadManager = (*mediaapp.Service)(nil)
