package gorm

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/korwlik23/go-api-starter/internal/modules/publishing/ports"
	"github.com/korwlik23/go-api-starter/internal/platform/databaseid"
	gormdb "gorm.io/gorm"
)

var ErrInvalidPreviewRepository = errors.New("invalid preview repository")

type PreviewRepository struct {
	db *gormdb.DB
}

type previewRecord struct {
	ID             databaseid.ID `gorm:"column:id"`
	UserID         databaseid.ID `gorm:"column:user_id"`
	AccountID      databaseid.ID `gorm:"column:account_id"`
	ContentID      databaseid.ID `gorm:"column:content_id"`
	TranslationID  databaseid.ID `gorm:"column:translation_id"`
	RevisionID     databaseid.ID `gorm:"column:revision_id"`
	TokenHash      []byte        `gorm:"column:token_hash"`
	HashVersion    int16         `gorm:"column:hash_version"`
	HashKeyVersion int64         `gorm:"column:hash_key_version"`
	Audience       string        `gorm:"column:audience"`
	ExpiresAt      time.Time     `gorm:"column:expires_at"`
	ConsumedAt     *time.Time    `gorm:"column:consumed_at"`
	CreatedAt      time.Time     `gorm:"column:created_at"`
}

func (previewRecord) TableName() string { return "publishing_preview_tokens" }

func NewPreviewRepository(db *gormdb.DB) (*PreviewRepository, error) {
	if db == nil {
		return nil, ErrInvalidPreviewRepository
	}
	return &PreviewRepository{db: db}, nil
}

func (repository *PreviewRepository) Create(ctx context.Context, token ports.PreviewToken) error {
	if repository == nil || repository.db == nil || ctx == nil {
		return ErrInvalidPreviewRepository
	}
	if err := ports.ValidatePreviewToken(token); err != nil {
		return err
	}
	ids, err := repository.encodeIDs(token)
	if err != nil {
		return ports.ErrInvalidPreviewRepository
	}
	return repository.db.WithContext(ctx).Table("publishing_preview_tokens").Create(map[string]any{
		"id": ids[0], "user_id": ids[1], "account_id": ids[2], "content_id": ids[3],
		"translation_id": ids[4], "revision_id": ids[5], "token_hash": token.TokenHash[:],
		"hash_version": token.HashVersion, "hash_key_version": token.HashKeyVersion,
		"audience": token.Audience, "expires_at": token.ExpiresAt,
		"consumed_at": nil, "created_at": token.CreatedAt,
	}).Error
}

func (repository *PreviewRepository) Find(ctx context.Context, tokenID string) (ports.PreviewToken, error) {
	if repository == nil || repository.db == nil || ctx == nil {
		return ports.PreviewToken{}, ErrInvalidPreviewRepository
	}
	encodedID, err := databaseid.Encode(repository.db.Dialector.Name(), tokenID)
	if err != nil {
		return ports.PreviewToken{}, ports.ErrPreviewNotFound
	}
	var record previewRecord
	result := repository.db.WithContext(ctx).Where("id = ?", encodedID).Limit(1).Find(&record)
	if result.Error != nil {
		return ports.PreviewToken{}, fmt.Errorf("find preview token: %w", result.Error)
	}
	if result.RowsAffected != 1 {
		return ports.PreviewToken{}, ports.ErrPreviewNotFound
	}
	return mapPreviewRecord(record)
}

func (repository *PreviewRepository) Consume(
	ctx context.Context,
	tokenID string,
	tokenHash [32]byte,
	now time.Time,
) error {
	if repository == nil || repository.db == nil || ctx == nil || now.IsZero() {
		return ErrInvalidPreviewRepository
	}
	encodedID, err := databaseid.Encode(repository.db.Dialector.Name(), tokenID)
	if err != nil || tokenHash == [32]byte{} {
		return ports.ErrPreviewConflict
	}
	err = repository.db.WithContext(ctx).Transaction(func(tx *gormdb.DB) error {
		result := tx.Table("publishing_preview_tokens").
			Where(
				"id = ? AND token_hash = ? AND consumed_at IS NULL "+
					"AND created_at <= ? AND expires_at > ?",
				encodedID, tokenHash[:], now, now,
			).
			Update("consumed_at", now)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return ports.ErrPreviewConflict
		}
		return nil
	})
	if err != nil {
		if errors.Is(err, ports.ErrPreviewConflict) {
			return err
		}
		return fmt.Errorf("consume preview token: %w", err)
	}
	return nil
}

func (repository *PreviewRepository) encodeIDs(token ports.PreviewToken) ([6]any, error) {
	values := [6]string{token.ID, token.UserID, token.AccountID, token.ContentID, token.TranslationID, token.RevisionID}
	encoded := [6]any{}
	for index, value := range values {
		id, err := databaseid.Encode(repository.db.Dialector.Name(), value)
		if err != nil {
			return [6]any{}, err
		}
		encoded[index] = id
	}
	return encoded, nil
}

func mapPreviewRecord(record previewRecord) (ports.PreviewToken, error) {
	if record.ID.String() == "" || record.UserID.String() == "" || record.AccountID.String() == "" ||
		record.ContentID.String() == "" || record.TranslationID.String() == "" || record.RevisionID.String() == "" ||
		len(record.TokenHash) != 32 {
		return ports.PreviewToken{}, ErrInvalidPreviewRepository
	}
	var hash [32]byte
	copy(hash[:], record.TokenHash)
	token := ports.PreviewToken{
		ID: record.ID.String(), UserID: record.UserID.String(), AccountID: record.AccountID.String(),
		ContentID: record.ContentID.String(), TranslationID: record.TranslationID.String(), RevisionID: record.RevisionID.String(),
		TokenHash: hash, HashVersion: record.HashVersion, HashKeyVersion: record.HashKeyVersion,
		Audience: record.Audience, ExpiresAt: record.ExpiresAt.UTC(), CreatedAt: record.CreatedAt.UTC(),
	}
	if err := ports.ValidatePreviewToken(token); err != nil {
		return ports.PreviewToken{}, err
	}
	return token, nil
}

var _ ports.PreviewRepository = (*PreviewRepository)(nil)
