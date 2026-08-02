package application

import (
	"context"
	"errors"
	"io"
	"time"

	mediadomain "github.com/korwlik23/go-api-starter/internal/modules/media/domain"
	mediaports "github.com/korwlik23/go-api-starter/internal/modules/media/ports"
	platformstorage "github.com/korwlik23/go-api-starter/internal/platform/storage"
)

var (
	ErrInvalidService       = errors.New("media: invalid service")
	ErrInvalidUploadCommand = errors.New("media: invalid upload command")
	ErrInvalidAssetCommand  = errors.New("media: invalid asset command")
	ErrInvalidUploadContent = errors.New("media: upload content failed validation")
	ErrDuplicateFinalize    = errors.New("media: upload already finalized")
	ErrUploadExpired        = errors.New("media: upload expired")
	ErrStorageCleanup       = errors.New("media: storage cleanup failed")
)

type MediaAuditEvent struct {
	ActorUserID string
	AccountID   string
	RequestID   string
	Action      string
	UploadID    string
	AssetID     string
	Version     int64
	Data        map[string]any
}

type MediaAuditor interface {
	RecordMediaChange(context.Context, MediaAuditEvent) error
}

type Dependencies struct {
	Repository mediaports.AssetRepository
	Storage    platformstorage.ObjectStore
	Auditor    MediaAuditor
	NewID      func() (string, error)
	Now        func() time.Time
	UploadTTL  time.Duration
	MaxSize    int64
}

type Service struct {
	dependencies Dependencies
}

func NewService(dependencies Dependencies) (*Service, error) {
	if dependencies.Repository == nil || dependencies.Storage == nil || dependencies.Auditor == nil || dependencies.NewID == nil || dependencies.Now == nil {
		return nil, ErrInvalidService
	}
	if dependencies.UploadTTL <= 0 {
		dependencies.UploadTTL = 15 * time.Minute
	}
	if dependencies.MaxSize <= 0 || dependencies.MaxSize > mediadomain.MaxAssetSize {
		dependencies.MaxSize = mediadomain.MaxAssetSize
	}
	return &Service{dependencies: dependencies}, nil
}

type StartUploadInput struct {
	AccountID      string
	ActorUserID    string
	RequestID      string
	OriginalName   string
	MIMEType       string
	DeclaredSize   int64
	ExpectedSHA256 string
	Body           io.Reader
}

type FinalizeUploadInput struct {
	AccountID       string
	ActorUserID     string
	RequestID       string
	UploadID        string
	ExpectedVersion int64
}

type ListAssetsInput struct {
	AccountID string
	After     *mediaports.AssetCursor
	Limit     int
}

type DeleteAssetInput struct {
	AccountID       string
	ActorUserID     string
	RequestID       string
	AssetID         string
	ExpectedVersion int64
}

func (service *Service) validMutation(accountID, actorID, requestID string) bool {
	return service != nil && service.dependencies.Repository != nil && service.dependencies.Storage != nil && service.dependencies.Auditor != nil &&
		service.dependencies.Now != nil && accountID != "" && actorID != "" && requestID != ""
}

func (service *Service) markUploadFailed(ctx context.Context, upload mediadomain.Upload, state mediadomain.UploadState, now time.Time) {
	_ = service.dependencies.Repository.FailUpload(ctx, mediaports.FailUploadCommand{
		AccountID: upload.AccountID, UploadID: upload.ID, ExpectedVersion: upload.Version,
		State: state, UpdatedAt: now,
	})
	_ = service.dependencies.Storage.Delete(ctx, upload.StorageKey)
}

func (service *Service) record(ctx context.Context, event MediaAuditEvent) error {
	return service.dependencies.Auditor.RecordMediaChange(ctx, event)
}
