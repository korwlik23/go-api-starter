package domain

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestM3B2UploadStateAndExpiryValidation(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 2, 11, 0, 0, 0, time.UTC)
	upload, err := NewUpload(UploadInput{
		ID: "018f1f64-7b2a-7c10-8bf2-3c40d5f67585", AccountID: "018f1f64-7b2a-7c10-8bf2-3c40d5f67586",
		StorageKey: "018f1f647b2a7c108bf23c40d5f67585.blob", OriginalName: "hero.png",
		MIMEType: "image/png", DeclaredSize: 8, ExpectedSHA256: "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
		State: UploadPending, Version: 1, ExpiresAt: now.Add(time.Hour), CreatedAt: now, UpdatedAt: now,
	})
	require.NoError(t, err)
	require.Equal(t, UploadPending, upload.State)
	require.True(t, upload.ExpiresAt.After(upload.CreatedAt))

	_, err = NewUpload(UploadInput{
		ID: upload.ID, AccountID: upload.AccountID, StorageKey: upload.StorageKey, OriginalName: "hero.png",
		MIMEType: "image/png", DeclaredSize: 8, ExpectedSHA256: upload.ExpectedSHA256,
		AssetID: "018f1f64-7b2a-7c10-8bf2-3c40d5f67590",
		State:   UploadFinalized, Version: 1, ExpiresAt: now.Add(time.Hour), CreatedAt: now, UpdatedAt: now,
	})
	require.NoError(t, err)
	_, err = NewUpload(UploadInput{
		ID: upload.ID, AccountID: upload.AccountID, StorageKey: upload.StorageKey, OriginalName: "hero.png",
		MIMEType: "image/png", DeclaredSize: 8, ExpectedSHA256: upload.ExpectedSHA256,
		State: UploadPending, Version: 1, ExpiresAt: now.Add(-time.Hour), CreatedAt: now, UpdatedAt: now,
	})
	require.ErrorIs(t, err, ErrInvalidUpload)
}
