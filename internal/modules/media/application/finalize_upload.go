package application

import (
	"context"
	"io"

	mediadomain "github.com/korwlik23/go-api-starter/internal/modules/media/domain"
	mediaports "github.com/korwlik23/go-api-starter/internal/modules/media/ports"
)

func (service *Service) FinalizeUpload(ctx context.Context, input FinalizeUploadInput) (mediadomain.Asset, error) {
	if ctx == nil || !service.validMutation(input.AccountID, input.ActorUserID, input.RequestID) || input.UploadID == "" || input.ExpectedVersion < 1 {
		return mediadomain.Asset{}, ErrInvalidUploadCommand
	}
	upload, err := service.dependencies.Repository.GetUpload(ctx, input.AccountID, input.UploadID)
	if err != nil {
		return mediadomain.Asset{}, err
	}
	if upload.State == mediadomain.UploadFinalized {
		return mediadomain.Asset{}, ErrDuplicateFinalize
	}
	if upload.State != mediadomain.UploadPending || upload.Version != input.ExpectedVersion {
		return mediadomain.Asset{}, mediaports.ErrUploadConflict
	}
	now := service.dependencies.Now().UTC()
	if now.IsZero() {
		return mediadomain.Asset{}, ErrInvalidUploadCommand
	}
	if !now.Before(upload.ExpiresAt) {
		service.markUploadFailed(ctx, upload, mediadomain.UploadExpired, now)
		return mediadomain.Asset{}, ErrUploadExpired
	}
	reader, object, err := service.dependencies.Storage.Open(ctx, upload.StorageKey)
	if err != nil {
		return mediadomain.Asset{}, err
	}
	data, readErr := io.ReadAll(io.LimitReader(reader, service.dependencies.MaxSize+1))
	closeErr := reader.Close()
	if readErr != nil {
		return mediadomain.Asset{}, readErr
	}
	if closeErr != nil {
		return mediadomain.Asset{}, closeErr
	}
	if int64(len(data)) > service.dependencies.MaxSize || int64(len(data)) != upload.DeclaredSize || (object.Size > 0 && object.Size != int64(len(data))) || mediadomain.SHA256Hex(data) != upload.ExpectedSHA256 || mediadomain.ValidateMagic(upload.MIMEType, data) != nil {
		service.markUploadFailed(ctx, upload, mediadomain.UploadFailed, now)
		return mediadomain.Asset{}, ErrInvalidUploadContent
	}
	assetID, err := service.dependencies.NewID()
	if err != nil {
		return mediadomain.Asset{}, err
	}
	width, height := mediadomain.DetectDimensions(upload.MIMEType, data)
	asset, err := mediadomain.NewAsset(mediadomain.AssetInput{
		ID: assetID, AccountID: upload.AccountID, StorageKey: upload.StorageKey, OriginalName: upload.OriginalName,
		MIMEType: upload.MIMEType, ByteSize: int64(len(data)), SHA256: mediadomain.SHA256Hex(data), Width: width, Height: height,
		Version: 1, CreatedAt: now, UpdatedAt: now,
	})
	if err != nil {
		return mediadomain.Asset{}, ErrInvalidUploadContent
	}
	result, err := service.dependencies.Repository.FinalizeUpload(ctx, mediaports.FinalizeAssetCommand{AccountID: upload.AccountID, UploadID: upload.ID, ExpectedVersion: upload.Version, Asset: asset, FinalizedAt: now})
	if err != nil {
		return mediadomain.Asset{}, err
	}
	if err := service.record(ctx, MediaAuditEvent{ActorUserID: input.ActorUserID, AccountID: upload.AccountID, RequestID: input.RequestID, Action: "media.upload.finalized", UploadID: upload.ID, AssetID: result.ID, Version: result.Version, Data: map[string]any{"mime_type": result.MIMEType, "byte_size": result.ByteSize, "sha256": result.SHA256}}); err != nil {
		_, _ = service.dependencies.Repository.SoftDeleteAsset(ctx, mediaports.DeleteAssetCommand{AccountID: result.AccountID, AssetID: result.ID, ExpectedVersion: result.Version, DeletedAt: now})
		return mediadomain.Asset{}, err
	}
	return result, nil
}
