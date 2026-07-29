package gorm

import (
	"context"
	"errors"
	"fmt"
	"time"

	localizationdomain "github.com/korwlik23/go-api-starter/internal/modules/localization/domain"
	"github.com/korwlik23/go-api-starter/internal/modules/localization/ports"
	"github.com/korwlik23/go-api-starter/internal/platform/databaseid"
	gormdb "gorm.io/gorm"
)

var ErrInvalidLocaleRepository = errors.New("invalid locale repository")

type LocaleRepository struct {
	db *gormdb.DB
}

type localeRecord struct {
	ID               databaseid.ID  `gorm:"column:id"`
	Tag              string         `gorm:"column:tag"`
	Name             string         `gorm:"column:name"`
	Direction        string         `gorm:"column:direction"`
	FallbackLocaleID *databaseid.ID `gorm:"column:fallback_locale_id"`
	Enabled          bool           `gorm:"column:enabled"`
	UserSelectable   bool           `gorm:"column:user_selectable"`
	DefaultSlot      *int16         `gorm:"column:default_slot"`
	Version          int64          `gorm:"column:version"`
	CreatedAt        time.Time      `gorm:"column:created_at"`
	UpdatedAt        time.Time      `gorm:"column:updated_at"`
}

func (localeRecord) TableName() string {
	return "locales"
}

func NewLocaleRepository(db *gormdb.DB) (*LocaleRepository, error) {
	if db == nil {
		return nil, ErrInvalidLocaleRepository
	}
	return &LocaleRepository{db: db}, nil
}

func (repository *LocaleRepository) ListEnabled(
	ctx context.Context,
) ([]localizationdomain.Locale, error) {
	var records []localeRecord
	err := repository.db.WithContext(ctx).
		Where("enabled = ?", true).
		Order("tag ASC").
		Find(&records).
		Error
	if err != nil {
		return nil, fmt.Errorf("list enabled locales: %w", err)
	}
	locales := make([]localizationdomain.Locale, 0, len(records))
	for _, record := range records {
		locale, mapErr := localeFromRecord(record)
		if mapErr != nil {
			return nil, mapErr
		}
		locales = append(locales, locale)
	}
	return locales, nil
}

func (repository *LocaleRepository) ListSelectable(
	ctx context.Context,
) ([]localizationdomain.Locale, error) {
	var records []localeRecord
	err := repository.db.WithContext(ctx).
		Where("enabled = ? AND user_selectable = ?", true, true).
		Order("tag ASC").
		Find(&records).
		Error
	if err != nil {
		return nil, fmt.Errorf("list selectable locales: %w", err)
	}
	locales := make([]localizationdomain.Locale, 0, len(records))
	for _, record := range records {
		locale, mapErr := localeFromRecord(record)
		if mapErr != nil {
			return nil, mapErr
		}
		locales = append(locales, locale)
	}
	return locales, nil
}

func localeFromRecord(
	record localeRecord,
) (localizationdomain.Locale, error) {
	tag, err := localizationdomain.NormalizeLocaleTag(record.Tag)
	if err != nil ||
		tag != record.Tag ||
		record.ID.String() == "" ||
		record.Version < 1 {
		return localizationdomain.Locale{}, ErrInvalidLocaleRepository
	}
	fallbackID := ""
	if record.FallbackLocaleID != nil {
		fallbackID = record.FallbackLocaleID.String()
	}
	return localizationdomain.Locale{
		ID:             record.ID.String(),
		Tag:            record.Tag,
		Name:           record.Name,
		Direction:      localizationdomain.Direction(record.Direction),
		Enabled:        record.Enabled,
		UserSelectable: record.UserSelectable,
		Default:        record.DefaultSlot != nil && *record.DefaultSlot == 1,
		FallbackID:     fallbackID,
		Version:        record.Version,
		CreatedAt:      record.CreatedAt,
		UpdatedAt:      record.UpdatedAt,
	}, nil
}

var _ ports.LocaleRepository = (*LocaleRepository)(nil)
var _ ports.SelectableLocaleRepository = (*LocaleRepository)(nil)
