package application

import (
	"context"
	"errors"

	mediaports "github.com/korwlik23/go-api-starter/internal/modules/media/ports"
)

func (service *Service) DeleteAsset(ctx context.Context, input DeleteAssetInput) error {
	if ctx == nil || !service.validMutation(input.AccountID, input.ActorUserID, input.RequestID) || input.AssetID == "" || input.ExpectedVersion < 1 {
		return ErrInvalidAssetCommand
	}
	now := service.dependencies.Now().UTC()
	if now.IsZero() {
		return ErrInvalidAssetCommand
	}
	asset, err := service.dependencies.Repository.SoftDeleteAsset(ctx, mediaports.DeleteAssetCommand{AccountID: input.AccountID, AssetID: input.AssetID, ExpectedVersion: input.ExpectedVersion, DeletedAt: now})
	if err != nil {
		return err
	}
	if err := service.record(ctx, MediaAuditEvent{ActorUserID: input.ActorUserID, AccountID: input.AccountID, RequestID: input.RequestID, Action: "media.asset.deleted", AssetID: asset.ID, Version: asset.Version, Data: map[string]any{"storage_key": asset.StorageKey}}); err != nil {
		return err
	}
	if err := service.dependencies.Storage.Delete(ctx, asset.StorageKey); err != nil {
		return errors.Join(ErrStorageCleanup, err)
	}
	return nil
}
