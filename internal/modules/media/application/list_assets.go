package application

import (
	"context"

	mediaports "github.com/korwlik23/go-api-starter/internal/modules/media/ports"
)

func (service *Service) ListAssets(ctx context.Context, input ListAssetsInput) (mediaports.AssetListPage, error) {
	if service == nil || ctx == nil {
		return mediaports.AssetListPage{}, ErrInvalidAssetCommand
	}
	if err := mediaports.ValidateAssetListQuery(mediaports.AssetListQuery{AccountID: input.AccountID, After: input.After, Limit: input.Limit}); err != nil {
		return mediaports.AssetListPage{}, err
	}
	return service.dependencies.Repository.ListAssets(ctx, mediaports.AssetListQuery{AccountID: input.AccountID, After: input.After, Limit: input.Limit})
}
