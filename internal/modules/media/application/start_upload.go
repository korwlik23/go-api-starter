package application

import (
	"context"
	"strings"

	mediadomain "github.com/korwlik23/go-api-starter/internal/modules/media/domain"
	platformstorage "github.com/korwlik23/go-api-starter/internal/platform/storage"
)

func (service *Service) StartUpload(ctx context.Context, input StartUploadInput) (mediadomain.Upload, error) {
	if ctx == nil || !service.validMutation(input.AccountID, input.ActorUserID, input.RequestID) || input.Body == nil || strings.TrimSpace(input.OriginalName) == "" || input.DeclaredSize < 0 || input.DeclaredSize > service.dependencies.MaxSize {
		return mediadomain.Upload{}, ErrInvalidUploadCommand
	}
	now := service.dependencies.Now().UTC()
	if now.IsZero() || !mediadomain.AllowedMIMEType(input.MIMEType) {
		return mediadomain.Upload{}, ErrInvalidUploadCommand
	}
	object, err := service.dependencies.Storage.Put(ctx, platformPutRequest(input))
	if err != nil {
		return mediadomain.Upload{}, err
	}
	uploadID, err := service.dependencies.NewID()
	if err != nil {
		_ = service.dependencies.Storage.Delete(ctx, object.Key)
		return mediadomain.Upload{}, err
	}
	upload, err := mediadomain.NewUpload(mediadomain.UploadInput{
		ID: uploadID, AccountID: input.AccountID, StorageKey: object.Key, OriginalName: input.OriginalName,
		MIMEType: input.MIMEType, DeclaredSize: input.DeclaredSize, ExpectedSHA256: input.ExpectedSHA256,
		State: mediadomain.UploadPending, Version: 1, ExpiresAt: now.Add(service.dependencies.UploadTTL), CreatedAt: now, UpdatedAt: now,
	})
	if err != nil {
		_ = service.dependencies.Storage.Delete(ctx, object.Key)
		return mediadomain.Upload{}, ErrInvalidUploadCommand
	}
	if err := service.dependencies.Repository.CreateUpload(ctx, upload); err != nil {
		_ = service.dependencies.Storage.Delete(ctx, object.Key)
		return mediadomain.Upload{}, err
	}
	if err := service.record(ctx, MediaAuditEvent{ActorUserID: input.ActorUserID, AccountID: input.AccountID, RequestID: input.RequestID, Action: "media.upload.started", UploadID: upload.ID, Version: upload.Version, Data: map[string]any{"mime_type": upload.MIMEType, "byte_size": upload.DeclaredSize}}); err != nil {
		service.markUploadFailed(ctx, upload, mediadomain.UploadFailed, now)
		return mediadomain.Upload{}, err
	}
	return upload, nil
}

func platformPutRequest(input StartUploadInput) platformstorage.PutRequest {
	return platformstorage.PutRequest{Body: input.Body, Size: input.DeclaredSize}
}
