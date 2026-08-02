package ports

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestM3B2AssetRepositoryQueryValidation(t *testing.T) {
	t.Parallel()

	require.NoError(t, ValidateAssetListQuery(AssetListQuery{
		AccountID: "018f1f64-7b2a-7c10-8bf2-3c40d5f67586", Limit: 25,
	}))
	require.ErrorIs(t, ValidateAssetListQuery(AssetListQuery{AccountID: "bad", Limit: 25}), ErrInvalidAssetQuery)
	require.ErrorIs(t, ValidateAssetListQuery(AssetListQuery{AccountID: "018f1f64-7b2a-7c10-8bf2-3c40d5f67586", Limit: 101}), ErrInvalidAssetQuery)
}
