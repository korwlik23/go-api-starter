package application

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	mediadomain "github.com/korwlik23/go-api-starter/internal/modules/media/domain"
	mediaports "github.com/korwlik23/go-api-starter/internal/modules/media/ports"
	platformstorage "github.com/korwlik23/go-api-starter/internal/platform/storage"
	"github.com/stretchr/testify/require"
)

func TestM3B2StartUploadCompensatesStorageWhenRepositoryFails(t *testing.T) {
	t.Parallel()

	storage := &storageFake{putInfo: platformstorage.ObjectInfo{Key: "018f1f647b2a7c108bf23c40d5f67585.blob", Size: 8}}
	repository := &assetRepositoryFake{createUploadErr: errors.New("database unavailable")}
	service, err := NewService(Dependencies{
		Repository: repository, Storage: storage, Auditor: &mediaAuditorFake{},
		NewID: func() (string, error) { return "018f1f64-7b2a-7c10-8bf2-3c40d5f67585", nil },
		Now:   func() time.Time { return time.Date(2026, 8, 2, 11, 0, 0, 0, time.UTC) },
	})
	require.NoError(t, err)
	_, err = service.StartUpload(context.Background(), StartUploadInput{
		AccountID: "018f1f64-7b2a-7c10-8bf2-3c40d5f67586", ActorUserID: "018f1f64-7b2a-7c10-8bf2-3c40d5f67587", RequestID: "req-1",
		OriginalName: "hero.png", MIMEType: "image/png", DeclaredSize: 8,
		ExpectedSHA256: mediadomain.SHA256Hex([]byte("\x89PNG\r\n\x1a\n")), Body: strings.NewReader("\x89PNG\r\n\x1a\n"),
	})
	require.Error(t, err)
	require.Equal(t, storage.putInfo.Key, storage.deletedKey)
	require.Nil(t, repository.upload)
}

func TestM3B2FinalizeRejectsMagicAndSizeMismatchAndMarksFailed(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 2, 11, 0, 0, 0, time.UTC)
	upload := testUpload(now, "image/png", 8, mediadomain.SHA256Hex([]byte("not-png")))
	service, err := NewService(Dependencies{
		Repository: &assetRepositoryFake{upload: &upload}, Storage: &storageFake{openBody: []byte("not-png")}, Auditor: &mediaAuditorFake{},
		NewID: func() (string, error) { return "018f1f64-7b2a-7c10-8bf2-3c40d5f67590", nil }, Now: func() time.Time { return now },
	})
	require.NoError(t, err)
	_, err = service.FinalizeUpload(context.Background(), FinalizeUploadInput{
		AccountID: upload.AccountID, ActorUserID: "018f1f64-7b2a-7c10-8bf2-3c40d5f67587", RequestID: "req-2", UploadID: upload.ID, ExpectedVersion: 1,
	})
	require.ErrorIs(t, err, ErrInvalidUploadContent)
}

func TestM3B2FinalizeIsIdempotentAndAccountScoped(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 2, 11, 0, 0, 0, time.UTC)
	body := []byte("\x89PNG\r\n\x1a\n")
	upload := testUpload(now, "image/png", int64(len(body)), mediadomain.SHA256Hex(body))
	upload.State = mediadomain.UploadFinalized
	upload.AssetID = "018f1f64-7b2a-7c10-8bf2-3c40d5f67590"
	repository := &assetRepositoryFake{upload: &upload}
	service, err := NewService(Dependencies{Repository: repository, Storage: &storageFake{openBody: body}, Auditor: &mediaAuditorFake{}, Now: func() time.Time { return now }, NewID: func() (string, error) { return "018f1f64-7b2a-7c10-8bf2-3c40d5f67591", nil }})
	require.NoError(t, err)
	_, err = service.FinalizeUpload(context.Background(), FinalizeUploadInput{AccountID: "018f1f64-7b2a-7c10-8bf2-3c40d5f67592", ActorUserID: "018f1f64-7b2a-7c10-8bf2-3c40d5f67587", RequestID: "req-3", UploadID: upload.ID, ExpectedVersion: 1})
	require.ErrorIs(t, err, mediaports.ErrUploadNotFound)
	_, err = service.FinalizeUpload(context.Background(), FinalizeUploadInput{AccountID: upload.AccountID, ActorUserID: "018f1f64-7b2a-7c10-8bf2-3c40d5f67587", RequestID: "req-4", UploadID: upload.ID, ExpectedVersion: 1})
	require.ErrorIs(t, err, ErrDuplicateFinalize)
}

func TestM3B2FinalizeValidatesHashAndCreatesAsset(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 2, 11, 0, 0, 0, time.UTC)
	body := []byte("\x89PNG\r\n\x1a\n")
	upload := testUpload(now, "image/png", int64(len(body)), mediadomain.SHA256Hex(body))
	repository := &assetRepositoryFake{upload: &upload}
	service, err := NewService(Dependencies{Repository: repository, Storage: &storageFake{openBody: body}, Auditor: &mediaAuditorFake{}, Now: func() time.Time { return now }, NewID: func() (string, error) { return "018f1f64-7b2a-7c10-8bf2-3c40d5f67590", nil }})
	require.NoError(t, err)
	asset, err := service.FinalizeUpload(context.Background(), FinalizeUploadInput{AccountID: upload.AccountID, ActorUserID: "018f1f64-7b2a-7c10-8bf2-3c40d5f67587", RequestID: "req-5", UploadID: upload.ID, ExpectedVersion: 1})
	require.NoError(t, err)
	require.Equal(t, upload.AccountID, asset.AccountID)
	require.Equal(t, mediadomain.SHA256Hex(body), asset.SHA256)
	require.Equal(t, mediadomain.UploadFinalized, repository.upload.State)
}

func TestM3B2DeleteSoftDeletesBeforeStorageCleanup(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 2, 11, 0, 0, 0, time.UTC)
	asset, err := mediadomain.NewAsset(mediadomain.AssetInput{ID: "018f1f64-7b2a-7c10-8bf2-3c40d5f67590", AccountID: "018f1f64-7b2a-7c10-8bf2-3c40d5f67586", StorageKey: "018f1f647b2a7c108bf23c40d5f67585.blob", OriginalName: "a.png", MIMEType: "image/png", ByteSize: 8, SHA256: mediadomain.SHA256Hex([]byte("\x89PNG\r\n\x1a\n")), Version: 1, CreatedAt: now, UpdatedAt: now})
	require.NoError(t, err)
	storage := &storageFake{deleteErr: errors.New("cleanup unavailable")}
	repository := &assetRepositoryFake{asset: &asset}
	service, err := NewService(Dependencies{Repository: repository, Storage: storage, Auditor: &mediaAuditorFake{}, Now: func() time.Time { return now }, NewID: func() (string, error) { return "018f1f64-7b2a-7c10-8bf2-3c40d5f67591", nil }})
	require.NoError(t, err)
	err = service.DeleteAsset(context.Background(), DeleteAssetInput{AccountID: asset.AccountID, ActorUserID: "018f1f64-7b2a-7c10-8bf2-3c40d5f67587", RequestID: "req-6", AssetID: asset.ID, ExpectedVersion: 1})
	require.ErrorIs(t, err, ErrStorageCleanup)
	require.NotNil(t, repository.deleted)
	// The DB soft-delete remains authoritative even when physical cleanup is uncertain.
	require.Equal(t, asset.ID, repository.deleted.ID)
}

func testUpload(now time.Time, mime string, size int64, sha string) mediadomain.Upload {
	upload, _ := mediadomain.NewUpload(mediadomain.UploadInput{ID: "018f1f64-7b2a-7c10-8bf2-3c40d5f67585", AccountID: "018f1f64-7b2a-7c10-8bf2-3c40d5f67586", StorageKey: "018f1f647b2a7c108bf23c40d5f67585.blob", OriginalName: "a.png", MIMEType: mime, DeclaredSize: size, ExpectedSHA256: sha, State: mediadomain.UploadPending, Version: 1, ExpiresAt: now.Add(time.Hour), CreatedAt: now, UpdatedAt: now})
	return upload
}

type storageFake struct {
	putInfo    platformstorage.ObjectInfo
	openBody   []byte
	deletedKey string
	deleteErr  error
}

func (fake *storageFake) Put(context.Context, platformstorage.PutRequest) (platformstorage.ObjectInfo, error) {
	return fake.putInfo, nil
}
func (fake *storageFake) Stat(context.Context, string) (platformstorage.ObjectInfo, error) {
	return fake.putInfo, nil
}
func (fake *storageFake) Delete(_ context.Context, key string) error {
	fake.deletedKey = key
	return fake.deleteErr
}
func (fake *storageFake) Open(context.Context, string) (io.ReadCloser, platformstorage.ObjectInfo, error) {
	return io.NopCloser(strings.NewReader(string(fake.openBody))), platformstorage.ObjectInfo{Size: int64(len(fake.openBody))}, nil
}

type assetRepositoryFake struct {
	upload          *mediadomain.Upload
	asset           *mediadomain.Asset
	deleted         *mediadomain.Asset
	createUploadErr error
}

func (fake *assetRepositoryFake) CreateUpload(_ context.Context, upload mediadomain.Upload) error {
	if fake.createUploadErr != nil {
		return fake.createUploadErr
	}
	fake.upload = &upload
	return nil
}
func (fake *assetRepositoryFake) GetUpload(_ context.Context, accountID, _ string) (mediadomain.Upload, error) {
	if fake.upload == nil || fake.upload.AccountID != accountID {
		return mediadomain.Upload{}, mediaports.ErrUploadNotFound
	}
	return *fake.upload, nil
}
func (fake *assetRepositoryFake) FailUpload(_ context.Context, command mediaports.FailUploadCommand) error {
	if fake.upload == nil {
		return mediaports.ErrUploadNotFound
	}
	fake.upload.State = command.State
	fake.upload.Version++
	return nil
}
func (fake *assetRepositoryFake) FinalizeUpload(_ context.Context, command mediaports.FinalizeAssetCommand) (mediadomain.Asset, error) {
	if fake.upload == nil {
		return mediadomain.Asset{}, mediaports.ErrUploadNotFound
	}
	if fake.upload.State != mediadomain.UploadPending || fake.upload.Version != command.ExpectedVersion {
		return mediadomain.Asset{}, mediaports.ErrUploadConflict
	}
	fake.upload.State = mediadomain.UploadFinalized
	fake.upload.AssetID = command.Asset.ID
	fake.upload.Version++
	fake.asset = &command.Asset
	return command.Asset, nil
}
func (fake *assetRepositoryFake) ListAssets(context.Context, mediaports.AssetListQuery) (mediaports.AssetListPage, error) {
	return mediaports.AssetListPage{}, nil
}
func (fake *assetRepositoryFake) SoftDeleteAsset(_ context.Context, command mediaports.DeleteAssetCommand) (mediadomain.Asset, error) {
	if fake.asset == nil || fake.asset.ID != command.AssetID {
		return mediadomain.Asset{}, mediaports.ErrAssetNotFound
	}
	deleted := *fake.asset
	deleted.DeletedAt = &command.DeletedAt
	fake.deleted = &deleted
	return deleted, nil
}

type mediaAuditorFake struct{ event MediaAuditEvent }

func (fake *mediaAuditorFake) RecordMediaChange(_ context.Context, event MediaAuditEvent) error {
	fake.event = event
	return nil
}
