package gorm

import (
	"context"
	"errors"
	"fmt"
	"time"

	identitydomain "github.com/korwlik23/go-api-starter/internal/modules/identity/domain"
	"github.com/korwlik23/go-api-starter/internal/modules/identity/ports"
	gormdb "gorm.io/gorm"
)

var ErrInvalidUserRepository = errors.New("invalid user repository")

type UserRepository struct {
	db *gormdb.DB
}

type userRecord struct {
	ID                   databaseID `gorm:"column:id"`
	NormalizedEmail      string     `gorm:"column:normalized_email"`
	Status               string     `gorm:"column:status"`
	AuthorizationVersion int64      `gorm:"column:authorization_version"`
	CreatedAt            time.Time  `gorm:"column:created_at"`
	UpdatedAt            time.Time  `gorm:"column:updated_at"`
}

func (userRecord) TableName() string {
	return "users"
}

func NewUserRepository(db *gormdb.DB) (*UserRepository, error) {
	if db == nil {
		return nil, ErrInvalidUserRepository
	}
	return &UserRepository{db: db}, nil
}

func (repository *UserRepository) FindByNormalizedEmail(
	ctx context.Context,
	email string,
) (identitydomain.User, error) {
	var record userRecord
	err := repository.db.WithContext(ctx).
		Select("id", "normalized_email", "status", "authorization_version").
		Where("normalized_email = ?", identitydomain.NormalizeEmail(email)).
		First(&record).
		Error
	if errors.Is(err, gormdb.ErrRecordNotFound) {
		return identitydomain.User{}, ports.ErrUserNotFound
	}
	if err != nil {
		return identitydomain.User{}, fmt.Errorf("find user by email: %w", err)
	}
	return identitydomain.User{
		ID:                   record.ID.String(),
		Email:                record.NormalizedEmail,
		Status:               identitydomain.UserStatus(record.Status),
		AuthorizationVersion: record.AuthorizationVersion,
		CreatedAt:            record.CreatedAt,
		UpdatedAt:            record.UpdatedAt,
	}, nil
}

func (repository *UserRepository) CurrentAuthorizationVersion(
	ctx context.Context,
	userID string,
) (int64, error) {
	encodedID, err := encodeDatabaseID(repository.db.Dialector.Name(), userID)
	if err != nil {
		return 0, fmt.Errorf("encode user ID: %w", err)
	}
	var record userRecord
	err = repository.db.WithContext(ctx).
		Select("authorization_version").
		Where("id = ?", encodedID).
		First(&record).
		Error
	if errors.Is(err, gormdb.ErrRecordNotFound) {
		return 0, ports.ErrUserNotFound
	}
	if err != nil {
		return 0, fmt.Errorf("read authorization version: %w", err)
	}
	return record.AuthorizationVersion, nil
}

func (repository *UserRepository) FindByID(
	ctx context.Context,
	userID string,
) (identitydomain.User, error) {
	encodedID, err := encodeDatabaseID(repository.db.Dialector.Name(), userID)
	if err != nil {
		return identitydomain.User{}, ports.ErrUserNotFound
	}
	var record userRecord
	err = repository.db.WithContext(ctx).
		Select("id", "normalized_email", "status", "authorization_version").
		Where("id = ?", encodedID).
		First(&record).
		Error
	if errors.Is(err, gormdb.ErrRecordNotFound) {
		return identitydomain.User{}, ports.ErrUserNotFound
	}
	if err != nil {
		return identitydomain.User{}, fmt.Errorf("find user by ID: %w", err)
	}
	return userFromRecord(record), nil
}

func userFromRecord(record userRecord) identitydomain.User {
	return identitydomain.User{
		ID:                   record.ID.String(),
		Email:                record.NormalizedEmail,
		Status:               identitydomain.UserStatus(record.Status),
		AuthorizationVersion: record.AuthorizationVersion,
		CreatedAt:            record.CreatedAt,
		UpdatedAt:            record.UpdatedAt,
	}
}

var _ ports.UserRepository = (*UserRepository)(nil)
var _ ports.UserByIDReader = (*UserRepository)(nil)
var _ ports.AuthorizationVersionReader = (*UserRepository)(nil)
