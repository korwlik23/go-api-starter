package domain

import (
	"errors"
	"strings"
	"time"
)

type UploadState string

const (
	UploadPending   UploadState = "pending"
	UploadFinalized UploadState = "finalized"
	UploadFailed    UploadState = "failed"
	UploadExpired   UploadState = "expired"
)

var ErrInvalidUpload = errors.New("media: invalid upload")

type Upload struct {
	ID             string
	AccountID      string
	StorageKey     string
	OriginalName   string
	MIMEType       string
	DeclaredSize   int64
	ExpectedSHA256 string
	State          UploadState
	AssetID        string
	Version        int64
	ExpiresAt      time.Time
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

type UploadInput struct {
	ID             string
	AccountID      string
	StorageKey     string
	OriginalName   string
	MIMEType       string
	DeclaredSize   int64
	ExpectedSHA256 string
	State          UploadState
	AssetID        string
	Version        int64
	ExpiresAt      time.Time
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

func NewUpload(input UploadInput) (Upload, error) {
	mimeType := NormalizeMIMEType(input.MIMEType)
	sha := strings.ToLower(strings.TrimSpace(input.ExpectedSHA256))
	state := input.State
	if state == "" {
		state = UploadPending
	}
	if !validUUID(input.ID) || !validUUID(input.AccountID) || !validStorageKey(input.StorageKey) ||
		!validFileName(input.OriginalName) || !isAllowedMIME(mimeType) || input.DeclaredSize < 0 ||
		input.DeclaredSize > MaxAssetSize || !sha256Pattern.MatchString(sha) || !validUploadState(state) ||
		input.Version < 1 || input.ExpiresAt.IsZero() || input.CreatedAt.IsZero() ||
		input.ExpiresAt.Before(input.CreatedAt) ||
		(!input.UpdatedAt.IsZero() && input.UpdatedAt.Before(input.CreatedAt)) {
		return Upload{}, ErrInvalidUpload
	}
	if state == UploadFinalized && !validUUID(input.AssetID) {
		return Upload{}, ErrInvalidUpload
	}
	if state != UploadFinalized && input.AssetID != "" {
		return Upload{}, ErrInvalidUpload
	}
	return Upload{
		ID: input.ID, AccountID: input.AccountID, StorageKey: strings.TrimSpace(input.StorageKey),
		OriginalName: strings.TrimSpace(input.OriginalName), MIMEType: mimeType,
		DeclaredSize: input.DeclaredSize, ExpectedSHA256: sha, State: state, AssetID: input.AssetID,
		Version: input.Version, ExpiresAt: input.ExpiresAt.UTC(), CreatedAt: input.CreatedAt.UTC(), UpdatedAt: input.UpdatedAt.UTC(),
	}, nil
}

func ValidateUploadState(state UploadState) error {
	if !validUploadState(state) {
		return ErrInvalidUpload
	}
	return nil
}

func validUploadState(state UploadState) bool {
	switch state {
	case UploadPending, UploadFinalized, UploadFailed, UploadExpired:
		return true
	default:
		return false
	}
}
