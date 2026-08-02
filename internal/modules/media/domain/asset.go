package domain

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"regexp"
	"strings"
	"time"
	"unicode"

	"github.com/google/uuid"
)

const MaxAssetSize int64 = 50 * 1024 * 1024

var (
	ErrInvalidAsset      = errors.New("media: invalid asset")
	ErrInvalidMagic      = errors.New("media: invalid file signature")
	ErrInvalidDimensions = errors.New("media: invalid dimensions")
)

type Asset struct {
	ID           string
	AccountID    string
	StorageKey   string
	OriginalName string
	MIMEType     string
	ByteSize     int64
	SHA256       string
	Width        int64
	Height       int64
	Version      int64
	DeletedAt    *time.Time
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

type AssetInput struct {
	ID           string
	AccountID    string
	StorageKey   string
	OriginalName string
	MIMEType     string
	ByteSize     int64
	SHA256       string
	Width        int64
	Height       int64
	Version      int64
	DeletedAt    *time.Time
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

var sha256Pattern = regexp.MustCompile(`^[0-9a-f]{64}$`)

func NewAsset(input AssetInput) (Asset, error) {
	mimeType := NormalizeMIMEType(input.MIMEType)
	sha := strings.ToLower(strings.TrimSpace(input.SHA256))
	if !validUUID(input.ID) || !validUUID(input.AccountID) ||
		!validStorageKey(input.StorageKey) || !validFileName(input.OriginalName) ||
		!isAllowedMIME(mimeType) || input.ByteSize < 0 || input.ByteSize > MaxAssetSize ||
		!sha256Pattern.MatchString(sha) || input.Version < 1 ||
		ValidateDimensions(mimeType, input.Width, input.Height) != nil ||
		(!input.CreatedAt.IsZero() && !input.UpdatedAt.IsZero() && input.UpdatedAt.Before(input.CreatedAt)) {
		return Asset{}, ErrInvalidAsset
	}
	return Asset{
		ID: input.ID, AccountID: input.AccountID, StorageKey: strings.TrimSpace(input.StorageKey),
		OriginalName: strings.TrimSpace(input.OriginalName), MIMEType: mimeType,
		ByteSize: input.ByteSize, SHA256: sha, Width: input.Width, Height: input.Height,
		Version: input.Version, DeletedAt: cloneTime(input.DeletedAt),
		CreatedAt: input.CreatedAt.UTC(), UpdatedAt: input.UpdatedAt.UTC(),
	}, nil
}

func NormalizeMIMEType(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func AllowedMIMEType(value string) bool {
	return isAllowedMIME(NormalizeMIMEType(value))
}

func ValidateMagic(mimeType string, data []byte) error {
	mimeType = NormalizeMIMEType(mimeType)
	switch mimeType {
	case "image/jpeg":
		if len(data) >= 3 && data[0] == 0xff && data[1] == 0xd8 && data[2] == 0xff {
			return nil
		}
	case "image/png":
		if len(data) >= 8 && string(data[:8]) == "\x89PNG\r\n\x1a\n" {
			return nil
		}
	case "image/gif":
		if len(data) >= 6 && (string(data[:6]) == "GIF87a" || string(data[:6]) == "GIF89a") {
			return nil
		}
	case "image/webp":
		if len(data) >= 12 && string(data[:4]) == "RIFF" && string(data[8:12]) == "WEBP" {
			return nil
		}
	case "application/pdf":
		if len(data) >= 5 && string(data[:5]) == "%PDF-" {
			return nil
		}
	}
	return ErrInvalidMagic
}

func ValidateDimensions(mimeType string, width, height int64) error {
	if width < 0 || height < 0 || width > 100000 || height > 100000 || (width == 0) != (height == 0) {
		return ErrInvalidDimensions
	}
	if strings.HasPrefix(NormalizeMIMEType(mimeType), "image/") && (width == 0 || height == 0) {
		return nil
	}
	return nil
}

func DetectDimensions(mimeType string, data []byte) (int64, int64) {
	if !strings.HasPrefix(NormalizeMIMEType(mimeType), "image/") {
		return 0, 0
	}
	config, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil || config.Width <= 0 || config.Height <= 0 {
		return 0, 0
	}
	return int64(config.Width), int64(config.Height)
}

func SHA256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func validUUID(value string) bool {
	parsed, err := uuid.Parse(strings.TrimSpace(value))
	return err == nil && parsed != uuid.Nil
}

func validStorageKey(value string) bool {
	value = strings.TrimSpace(value)
	if value == "" || len(value) > 255 || strings.ContainsAny(value, "/\\") || strings.Contains(value, "..") {
		return false
	}
	for _, character := range value {
		if unicode.IsControl(character) {
			return false
		}
	}
	return true
}

func validFileName(value string) bool {
	value = strings.TrimSpace(value)
	if value == "" || len(value) > 255 || strings.ContainsAny(value, "/\\") {
		return false
	}
	for _, character := range value {
		if unicode.IsControl(character) {
			return false
		}
	}
	return true
}

func isAllowedMIME(value string) bool {
	switch value {
	case "image/jpeg", "image/png", "image/gif", "image/webp", "application/pdf":
		return true
	default:
		return false
	}
}

func cloneTime(value *time.Time) *time.Time {
	if value == nil {
		return nil
	}
	cloned := value.UTC()
	return &cloned
}
