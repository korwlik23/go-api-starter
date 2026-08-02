package domain

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestM3B2AssetValidatesPrivateMetadata(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 2, 11, 0, 0, 0, time.UTC)
	asset, err := NewAsset(AssetInput{
		ID: "018f1f64-7b2a-7c10-8bf2-3c40d5f67585", AccountID: "018f1f64-7b2a-7c10-8bf2-3c40d5f67586",
		StorageKey: "018f1f647b2a7c108bf23c40d5f67585.blob", OriginalName: "hero.png",
		MIMEType: "image/png", ByteSize: 8, SHA256: "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
		Width: 2, Height: 1, Version: 1, CreatedAt: now, UpdatedAt: now,
	})
	require.NoError(t, err)
	require.Equal(t, "image/png", asset.MIMEType)
	require.Equal(t, int64(8), asset.ByteSize)

	for _, input := range []AssetInput{
		{ID: asset.ID, AccountID: asset.AccountID, StorageKey: "../outside", OriginalName: "x", MIMEType: "image/png", ByteSize: 1, SHA256: asset.SHA256, Version: 1, CreatedAt: now, UpdatedAt: now},
		{ID: asset.ID, AccountID: asset.AccountID, StorageKey: asset.StorageKey, OriginalName: "x", MIMEType: "application/x-executable", ByteSize: 1, SHA256: asset.SHA256, Version: 1, CreatedAt: now, UpdatedAt: now},
		{ID: asset.ID, AccountID: asset.AccountID, StorageKey: asset.StorageKey, OriginalName: "x", MIMEType: "image/png", ByteSize: 1, SHA256: "bad", Version: 1, CreatedAt: now, UpdatedAt: now},
		{ID: asset.ID, AccountID: asset.AccountID, StorageKey: asset.StorageKey, OriginalName: "x", MIMEType: "image/png", ByteSize: -1, SHA256: asset.SHA256, Version: 1, CreatedAt: now, UpdatedAt: now},
	} {
		_, err := NewAsset(input)
		require.ErrorIs(t, err, ErrInvalidAsset)
	}
}

func TestM3B2MagicBytesAndDimensions(t *testing.T) {
	t.Parallel()

	require.NoError(t, ValidateMagic("image/png", []byte("\x89PNG\r\n\x1a\nrest")))
	require.NoError(t, ValidateMagic("application/pdf", []byte("%PDF-1.7")))
	require.ErrorIs(t, ValidateMagic("image/png", []byte("GIF89a")), ErrInvalidMagic)
	require.ErrorIs(t, ValidateMagic("image/jpeg", []byte("bad")), ErrInvalidMagic)
	require.NoError(t, ValidateDimensions("image/png", 1920, 1080))
	require.NoError(t, ValidateDimensions("application/pdf", 0, 0))
	require.ErrorIs(t, ValidateDimensions("image/png", 0, 10), ErrInvalidDimensions)
}
