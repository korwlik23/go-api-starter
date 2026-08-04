package gorm

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/korwlik23/go-api-starter/internal/modules/settings/domain"
	"github.com/korwlik23/go-api-starter/internal/modules/settings/ports"
	"github.com/korwlik23/go-api-starter/internal/platform/database"
	gormdb "gorm.io/gorm"
)

var ErrInvalidRepository = errors.New("settings: invalid repository")

type Repository struct {
	connections database.ConnectionProvider
}

type settingRecord struct {
	Key       string    `gorm:"column:key;primaryKey"`
	Value     string    `gorm:"column:value"`
	Version   int64     `gorm:"column:version"`
	CreatedAt time.Time `gorm:"column:created_at"`
	UpdatedAt time.Time `gorm:"column:updated_at"`
}

func (settingRecord) TableName() string { return "system_settings" }

func NewRepository(db *gormdb.DB) (*Repository, error) {
	if db == nil {
		return nil, ErrInvalidRepository
	}
	return &Repository{connections: databaseConnection{db: db}}, nil
}

// NewRepositoryWithDependencies is used by application composition so a
// mutation participates in the shared transaction boundary.
func NewRepositoryWithDependencies(connections database.ConnectionProvider) (*Repository, error) {
	if connections == nil {
		return nil, ErrInvalidRepository
	}
	return &Repository{connections: connections}, nil
}

type databaseConnection struct{ db *gormdb.DB }

func (provider databaseConnection) Connection(ctx context.Context) (*gormdb.DB, error) {
	if provider.db == nil || ctx == nil {
		return nil, ErrInvalidRepository
	}
	return provider.db.WithContext(ctx), nil
}

func (repository *Repository) connection(ctx context.Context) (*gormdb.DB, error) {
	if repository == nil || repository.connections == nil || ctx == nil {
		return nil, ErrInvalidRepository
	}
	return repository.connections.Connection(ctx)
}

func (repository *Repository) List(ctx context.Context) ([]domain.Setting, error) {
	db, err := repository.connection(ctx)
	if err != nil {
		return nil, err
	}
	var records []settingRecord
	if err := db.Order("key ASC").Find(&records).Error; err != nil {
		return nil, fmt.Errorf("list settings: %w", err)
	}
	items := make([]domain.Setting, 0, len(records))
	for _, record := range records {
		item, mapErr := fromRecord(record)
		if mapErr != nil {
			return nil, mapErr
		}
		items = append(items, item)
	}
	return items, nil
}

func (repository *Repository) Find(ctx context.Context, key string) (domain.Setting, error) {
	db, err := repository.connection(ctx)
	if err != nil {
		return domain.Setting{}, err
	}
	var record settingRecord
	if err := db.Where("key = ?", key).First(&record).Error; err != nil {
		if errors.Is(err, gormdb.ErrRecordNotFound) {
			return domain.Setting{}, ports.ErrNotFound
		}
		return domain.Setting{}, fmt.Errorf("find setting: %w", err)
	}
	return fromRecord(record)
}

func (repository *Repository) Save(ctx context.Context, setting domain.Setting, expectedVersion int64) error {
	db, err := repository.connection(ctx)
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	if setting.Version < 1 {
		return domain.ErrInvalidSetting
	}
	if expectedVersion == 0 {
		record := settingRecord{Key: setting.Key, Value: setting.Value, Version: setting.Version, CreatedAt: now, UpdatedAt: now}
		if err := db.Create(&record).Error; err != nil {
			return fmt.Errorf("create setting: %w", err)
		}
		return nil
	}
	result := db.Model(&settingRecord{}).Where("key = ? AND version = ?", setting.Key, expectedVersion).Updates(map[string]any{
		"value": setting.Value, "version": setting.Version, "updated_at": now,
	})
	if result.Error != nil {
		return fmt.Errorf("update setting: %w", result.Error)
	}
	if result.RowsAffected != 1 {
		return domain.ErrVersionConflict
	}
	return nil
}

func fromRecord(record settingRecord) (domain.Setting, error) {
	if err := domain.ValidateKey(record.Key); err != nil || record.Version < 1 {
		return domain.Setting{}, domain.ErrInvalidSetting
	}
	return domain.Setting{Key: record.Key, Value: record.Value, Version: record.Version}, nil
}

var _ ports.Repository = (*Repository)(nil)
