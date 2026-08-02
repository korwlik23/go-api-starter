package ports

import (
	"context"
	"errors"
	"regexp"
	"time"

	mediadomain "github.com/korwlik23/go-api-starter/internal/modules/media/domain"
)

var (
	ErrInvalidAssetRepository = errors.New("invalid media asset repository")
	ErrInvalidAssetQuery      = errors.New("invalid media asset query")
	ErrAssetNotFound          = errors.New("media asset not found")
	ErrUploadNotFound         = errors.New("media upload not found")
	ErrAssetConflict          = errors.New("media asset version conflict")
	ErrUploadConflict         = errors.New("media upload version conflict")
)

var assetIDPattern = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

type AssetCursor struct {
	UpdatedAt time.Time
	ID        string
}

type AssetListQuery struct {
	AccountID string
	After     *AssetCursor
	Limit     int
}

type AssetListPage struct {
	Items []mediadomain.Asset
	Next  *AssetCursor
}

type FinalizeAssetCommand struct {
	AccountID       string
	UploadID        string
	ExpectedVersion int64
	Asset           mediadomain.Asset
	FinalizedAt     time.Time
}

type DeleteAssetCommand struct {
	AccountID       string
	AssetID         string
	ExpectedVersion int64
	DeletedAt       time.Time
}

type FailUploadCommand struct {
	AccountID       string
	UploadID        string
	ExpectedVersion int64
	State           mediadomain.UploadState
	UpdatedAt       time.Time
}

type AssetRepository interface {
	CreateUpload(context.Context, mediadomain.Upload) error
	GetUpload(context.Context, string, string) (mediadomain.Upload, error)
	FailUpload(context.Context, FailUploadCommand) error
	FinalizeUpload(context.Context, FinalizeAssetCommand) (mediadomain.Asset, error)
	ListAssets(context.Context, AssetListQuery) (AssetListPage, error)
	SoftDeleteAsset(context.Context, DeleteAssetCommand) (mediadomain.Asset, error)
}

func ValidateAssetListQuery(query AssetListQuery) error {
	if !assetIDPattern.MatchString(query.AccountID) || query.Limit < 1 || query.Limit > 100 {
		return ErrInvalidAssetQuery
	}
	if query.After != nil && (query.After.UpdatedAt.IsZero() || !assetIDPattern.MatchString(query.After.ID)) {
		return ErrInvalidAssetQuery
	}
	return nil
}
