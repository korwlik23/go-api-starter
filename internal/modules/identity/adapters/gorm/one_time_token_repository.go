package gorm

import (
	"context"
	"errors"
	"fmt"

	identitydomain "github.com/korwlik23/go-api-starter/internal/modules/identity/domain"
	"github.com/korwlik23/go-api-starter/internal/modules/identity/ports"
	gormdb "gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var ErrInvalidOneTimeTokenRepository = errors.New(
	"invalid one-time token repository",
)

type OneTimeTokenRepository struct {
	db *gormdb.DB
}

type passwordResetMetadataRecord struct {
	ID                   databaseID `gorm:"column:id"`
	UserID               databaseID `gorm:"column:user_id"`
	HashVersion          int16      `gorm:"column:hash_version"`
	HashKeyVersion       int64      `gorm:"column:hash_key_version"`
	AuthorizationVersion int64      `gorm:"column:authorization_version"`
	MFARequired          bool       `gorm:"column:mfa_required"`
}

func (passwordResetMetadataRecord) TableName() string {
	return "one_time_tokens"
}

func NewOneTimeTokenRepository(
	db *gormdb.DB,
) (*OneTimeTokenRepository, error) {
	if db == nil {
		return nil, ErrInvalidOneTimeTokenRepository
	}
	return &OneTimeTokenRepository{db: db}, nil
}

func (repository *OneTimeTokenRepository) Issue(
	ctx context.Context,
	token identitydomain.OneTimeToken,
	delivery identitydomain.EncryptedTokenDelivery,
) error {
	if token.ID == "" ||
		token.UserID == "" ||
		token.Purpose == "" ||
		token.HashVersion < 1 ||
		token.HashKeyVersion < 1 ||
		!token.ExpiresAt.After(token.CreatedAt) ||
		delivery.TokenID != token.ID ||
		delivery.KeyVersion < 1 ||
		len(delivery.Nonce) != 12 ||
		len(delivery.Ciphertext) < 17 ||
		!delivery.ExpiresAt.After(delivery.CreatedAt) {
		return ErrInvalidOneTimeTokenRepository
	}
	tokenID, err := encodeDatabaseID(repository.db.Dialector.Name(), token.ID)
	if err != nil {
		return fmt.Errorf("encode one-time token ID: %w", err)
	}
	userID, err := encodeDatabaseID(repository.db.Dialector.Name(), token.UserID)
	if err != nil {
		return fmt.Errorf("encode one-time token user ID: %w", err)
	}

	err = repository.db.WithContext(ctx).Transaction(func(tx *gormdb.DB) error {
		var lockedUser userRecord
		if lockErr := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Select("id").
			Where("id = ?", userID).
			First(&lockedUser).
			Error; lockErr != nil {
			return lockErr
		}
		if revokeErr := tx.Model(&oneTimeTokenRecord{}).
			Where(
				"user_id = ? AND purpose = ? AND consumed_at IS NULL "+
					"AND revoked_at IS NULL AND expires_at > ?",
				userID,
				string(token.Purpose),
				token.CreatedAt,
			).
			Update("revoked_at", token.CreatedAt).
			Error; revokeErr != nil {
			return revokeErr
		}
		if createErr := tx.Table("one_time_tokens").
			Create(oneTimeTokenValues(tokenID, userID, token)).
			Error; createErr != nil {
			return createErr
		}
		return tx.Table("one_time_token_deliveries").
			Create(map[string]any{
				"token_id":         tokenID,
				"envelope_version": int16(1),
				"key_version":      delivery.KeyVersion,
				"nonce":            append([]byte(nil), delivery.Nonce...),
				"ciphertext":       append([]byte(nil), delivery.Ciphertext...),
				"expires_at":       delivery.ExpiresAt,
				"delivered_at":     nil,
				"created_at":       delivery.CreatedAt,
			}).
			Error
	})
	if err != nil {
		return fmt.Errorf("issue one-time token: %w", err)
	}
	return nil
}

type oneTimeTokenRecord struct {
	ID             databaseID `gorm:"column:id"`
	UserID         databaseID `gorm:"column:user_id"`
	Purpose        string     `gorm:"column:purpose"`
	TokenHash      []byte     `gorm:"column:token_hash"`
	HashVersion    int16      `gorm:"column:hash_version"`
	HashKeyVersion int64      `gorm:"column:hash_key_version"`
}

func (oneTimeTokenRecord) TableName() string {
	return "one_time_tokens"
}

func oneTimeTokenValues(
	tokenID any,
	userID any,
	token identitydomain.OneTimeToken,
) map[string]any {
	return map[string]any{
		"id":               tokenID,
		"user_id":          userID,
		"purpose":          string(token.Purpose),
		"token_hash":       token.TokenHash[:],
		"hash_version":     token.HashVersion,
		"hash_key_version": token.HashKeyVersion,
		"expires_at":       token.ExpiresAt,
		"consumed_at":      nil,
		"revoked_at":       nil,
		"created_at":       token.CreatedAt,
	}
}

func (repository *OneTimeTokenRepository) FindPasswordResetToken(
	ctx context.Context,
	tokenID string,
) (ports.PasswordResetTokenMetadata, error) {
	encodedID, err := encodeDatabaseID(repository.db.Dialector.Name(), tokenID)
	if err != nil {
		return ports.PasswordResetTokenMetadata{}, ports.ErrOneTimeTokenNotFound
	}
	var record passwordResetMetadataRecord
	err = repository.db.WithContext(ctx).
		Select(
			"one_time_tokens.id, one_time_tokens.user_id, "+
				"one_time_tokens.hash_version, one_time_tokens.hash_key_version, "+
				"users.authorization_version, "+
				"EXISTS (SELECT 1 FROM mfa_totp_credentials "+
				"WHERE mfa_totp_credentials.user_id = one_time_tokens.user_id "+
				"AND confirmed_at IS NOT NULL) AS mfa_required",
		).
		Joins("JOIN users ON users.id = one_time_tokens.user_id").
		Where(
			"one_time_tokens.id = ? AND one_time_tokens.purpose = ?",
			encodedID,
			string(identitydomain.OneTimeTokenPurposePasswordReset),
		).
		First(&record).
		Error
	if errors.Is(err, gormdb.ErrRecordNotFound) {
		return ports.PasswordResetTokenMetadata{}, ports.ErrOneTimeTokenNotFound
	}
	if err != nil {
		return ports.PasswordResetTokenMetadata{}, fmt.Errorf(
			"find password reset token: %w",
			err,
		)
	}
	return ports.PasswordResetTokenMetadata{
		ID:                   record.ID.String(),
		UserID:               record.UserID.String(),
		HashVersion:          record.HashVersion,
		HashKeyVersion:       record.HashKeyVersion,
		AuthorizationVersion: record.AuthorizationVersion,
		MFARequired:          record.MFARequired,
	}, nil
}

func (repository *OneTimeTokenRepository) CompletePasswordReset(
	ctx context.Context,
	command ports.CompletePasswordResetCommand,
) error {
	if command.Replacement.UserID == nil ||
		command.Replacement.AuthorizationVersion == nil ||
		command.NewPasswordHash == "" ||
		command.HashVersion < 1 {
		return ErrInvalidOneTimeTokenRepository
	}
	tokenID, err := encodeDatabaseID(repository.db.Dialector.Name(), command.TokenID)
	if err != nil {
		return ports.ErrOneTimeTokenConflict
	}
	userID, err := encodeDatabaseID(
		repository.db.Dialector.Name(),
		*command.Replacement.UserID,
	)
	if err != nil {
		return ErrInvalidOneTimeTokenRepository
	}
	newAuthorizationVersion := *command.Replacement.AuthorizationVersion

	err = repository.db.WithContext(ctx).Transaction(func(tx *gormdb.DB) error {
		result := tx.Model(&oneTimeTokenRecord{}).
			Where(
				"id = ? AND user_id = ? AND purpose = ? AND token_hash = ? "+
					"AND hash_version = ? AND consumed_at IS NULL "+
					"AND revoked_at IS NULL AND created_at <= ? AND expires_at > ?",
				tokenID,
				userID,
				string(identitydomain.OneTimeTokenPurposePasswordReset),
				command.CandidateHash[:],
				command.HashVersion,
				command.Now,
				command.Now,
			).
			Update("consumed_at", command.Now)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return ports.ErrOneTimeTokenConflict
		}

		result = tx.Model(&passwordCredentialRecord{}).
			Where("user_id = ?", userID).
			Updates(map[string]any{
				"password_hash": command.NewPasswordHash,
				"changed_at":    command.Now,
			})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return ports.ErrOneTimeTokenConflict
		}

		result = tx.Model(&userRecord{}).
			Where(
				"id = ? AND authorization_version = ?",
				userID,
				newAuthorizationVersion-1,
			).
			Update("authorization_version", newAuthorizationVersion)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return ports.ErrOneTimeTokenConflict
		}

		if revokeErr := tx.Model(&sessionRecord{}).
			Where("user_id = ? AND revoked_at IS NULL", userID).
			Update("revoked_at", command.Now).
			Error; revokeErr != nil {
			return revokeErr
		}
		if revokeErr := tx.Model(&oneTimeTokenRecord{}).
			Where(
				"user_id = ? AND id <> ? AND purpose = ? "+
					"AND consumed_at IS NULL AND revoked_at IS NULL",
				userID,
				tokenID,
				string(identitydomain.OneTimeTokenPurposePasswordReset),
			).
			Update("revoked_at", command.Now).
			Error; revokeErr != nil {
			return revokeErr
		}
		return createSession(tx, command.Replacement)
	})
	if err != nil {
		return fmt.Errorf("complete password reset: %w", err)
	}
	return nil
}

var _ ports.OneTimeTokenIssuer = (*OneTimeTokenRepository)(nil)
var _ ports.PasswordResetRepository = (*OneTimeTokenRepository)(nil)
