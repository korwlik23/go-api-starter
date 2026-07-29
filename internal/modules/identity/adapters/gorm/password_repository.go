package gorm

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/korwlik23/go-api-starter/internal/modules/identity/ports"
	gormdb "gorm.io/gorm"
)

var ErrInvalidPasswordRepository = errors.New("invalid password repository")

type PasswordRepository struct {
	db *gormdb.DB
}

type passwordCredentialRecord struct {
	UserID       databaseID `gorm:"column:user_id"`
	PasswordHash string     `gorm:"column:password_hash"`
	ChangedAt    time.Time  `gorm:"column:changed_at"`
}

func (passwordCredentialRecord) TableName() string {
	return "password_credentials"
}

func NewPasswordRepository(db *gormdb.DB) (*PasswordRepository, error) {
	if db == nil {
		return nil, ErrInvalidPasswordRepository
	}
	return &PasswordRepository{db: db}, nil
}

func (repository *PasswordRepository) FindPasswordHash(
	ctx context.Context,
	userID string,
) (string, error) {
	encodedID, err := encodeDatabaseID(repository.db.Dialector.Name(), userID)
	if err != nil {
		return "", fmt.Errorf("encode credential user ID: %w", err)
	}
	var record passwordCredentialRecord
	err = repository.db.WithContext(ctx).
		Select("password_hash").
		Where("user_id = ?", encodedID).
		First(&record).
		Error
	if errors.Is(err, gormdb.ErrRecordNotFound) {
		return "", ports.ErrPasswordNotFound
	}
	if err != nil {
		return "", fmt.Errorf("find password credential: %w", err)
	}
	return record.PasswordHash, nil
}

func (repository *PasswordRepository) ChangePassword(
	ctx context.Context,
	userID string,
	passwordHash string,
	changedAt time.Time,
) error {
	return repository.update(
		ctx,
		userID,
		map[string]any{
			"password_hash": passwordHash,
			"changed_at":    changedAt,
		},
	)
}

func (repository *PasswordRepository) RehashPassword(
	ctx context.Context,
	userID string,
	passwordHash string,
) error {
	return repository.update(
		ctx,
		userID,
		map[string]any{"password_hash": passwordHash},
	)
}

func (repository *PasswordRepository) update(
	ctx context.Context,
	userID string,
	values map[string]any,
) error {
	encodedID, err := encodeDatabaseID(repository.db.Dialector.Name(), userID)
	if err != nil {
		return fmt.Errorf("encode credential user ID: %w", err)
	}
	result := repository.db.WithContext(ctx).
		Model(&passwordCredentialRecord{}).
		Where("user_id = ?", encodedID).
		Updates(values)
	if result.Error != nil {
		return fmt.Errorf("update password credential: %w", result.Error)
	}
	if result.RowsAffected != 1 {
		return ports.ErrPasswordNotFound
	}
	return nil
}

var _ ports.PasswordRepository = (*PasswordRepository)(nil)
var _ ports.PasswordWriter = (*PasswordRepository)(nil)
