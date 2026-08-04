package gorm

import (
	"context"
	"errors"
	"fmt"
	"time"

	identitydomain "github.com/korwlik23/go-api-starter/internal/modules/identity/domain"
	"github.com/korwlik23/go-api-starter/internal/modules/identity/ports"
	"github.com/korwlik23/go-api-starter/internal/platform/database"
	gormdb "gorm.io/gorm"
)

var ErrInvalidProfileRepository = errors.New("invalid profile repository")

type ProfileRepository struct{ connections database.ConnectionProvider }
type profileRecord struct {
	UserID          databaseID `gorm:"column:user_id"`
	DisplayName     string     `gorm:"column:display_name"`
	Timezone        string     `gorm:"column:timezone"`
	PreferredLocale string     `gorm:"column:preferred_locale"`
	Version         int64      `gorm:"column:version"`
	CreatedAt       time.Time  `gorm:"column:created_at"`
	UpdatedAt       time.Time  `gorm:"column:updated_at"`
}

func (profileRecord) TableName() string { return "user_profiles" }

func NewProfileRepository(db *gormdb.DB) (*ProfileRepository, error) {
	if db == nil {
		return nil, ErrInvalidProfileRepository
	}
	return &ProfileRepository{connections: rootConnection{db: db}}, nil
}

type rootConnection struct{ db *gormdb.DB }

func (provider rootConnection) Connection(ctx context.Context) (*gormdb.DB, error) {
	if provider.db == nil || ctx == nil {
		return nil, ErrInvalidProfileRepository
	}
	return provider.db.WithContext(ctx), nil
}

func NewProfileRepositoryWithDependencies(connections database.ConnectionProvider) (*ProfileRepository, error) {
	if connections == nil {
		return nil, ErrInvalidProfileRepository
	}
	return &ProfileRepository{connections: connections}, nil
}
func (repository *ProfileRepository) connection(ctx context.Context) (*gormdb.DB, error) {
	if repository == nil || repository.connections == nil || ctx == nil {
		return nil, ErrInvalidProfileRepository
	}
	return repository.connections.Connection(ctx)
}
func (repository *ProfileRepository) FindProfile(ctx context.Context, userID string) (identitydomain.Profile, error) {
	db, err := repository.connection(ctx)
	if err != nil {
		return identitydomain.Profile{}, err
	}
	encoded, err := encodeDatabaseID(db.Dialector.Name(), userID)
	if err != nil {
		return identitydomain.Profile{}, ports.ErrProfileNotFound
	}
	var record profileRecord
	if err := db.Where("user_id = ?", encoded).First(&record).Error; err != nil {
		if errors.Is(err, gormdb.ErrRecordNotFound) {
			return identitydomain.Profile{}, ports.ErrProfileNotFound
		}
		return identitydomain.Profile{}, fmt.Errorf("find profile: %w", err)
	}
	return profileFromRecord(record), nil
}
func (repository *ProfileRepository) SaveProfile(ctx context.Context, profile identitydomain.Profile, expectedVersion int64) error {
	db, err := repository.connection(ctx)
	if err != nil {
		return err
	}
	encoded, err := encodeDatabaseID(db.Dialector.Name(), profile.UserID)
	if err != nil {
		return err
	}
	if expectedVersion == 0 {
		if err := db.Table("user_profiles").Create(map[string]any{"user_id": encoded, "display_name": profile.DisplayName, "timezone": profile.Timezone, "preferred_locale": profile.PreferredLocale, "version": profile.Version, "created_at": profile.CreatedAt, "updated_at": profile.UpdatedAt}).Error; err != nil {
			return fmt.Errorf("create profile: %w", err)
		}
		return nil
	}
	result := db.Model(&profileRecord{}).Where("user_id = ? AND version = ?", encoded, expectedVersion).Updates(map[string]any{"display_name": profile.DisplayName, "timezone": profile.Timezone, "preferred_locale": profile.PreferredLocale, "version": profile.Version, "updated_at": profile.UpdatedAt})
	if result.Error != nil {
		return fmt.Errorf("update profile: %w", result.Error)
	}
	if result.RowsAffected != 1 {
		return ports.ErrProfileConflict
	}
	return nil
}
func profileFromRecord(record profileRecord) identitydomain.Profile {
	return identitydomain.Profile{UserID: record.UserID.String(), DisplayName: record.DisplayName, Timezone: record.Timezone, PreferredLocale: record.PreferredLocale, Version: record.Version, CreatedAt: record.CreatedAt, UpdatedAt: record.UpdatedAt}
}

var _ ports.ProfileRepository = (*ProfileRepository)(nil)
