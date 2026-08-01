package gorm

import (
	"context"
	"errors"
	"fmt"
	"time"

	localizationdomain "github.com/korwlik23/go-api-starter/internal/modules/localization/domain"
	"github.com/korwlik23/go-api-starter/internal/modules/localization/ports"
	"github.com/korwlik23/go-api-starter/internal/platform/database"
	"github.com/korwlik23/go-api-starter/internal/platform/databaseid"
	gormdb "gorm.io/gorm"
)

var ErrInvalidLocaleRepository = errors.New("invalid locale repository")

type LocaleRepository struct {
	db           *gormdb.DB
	connections  database.ConnectionProvider
	transactions database.Transactor
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
	manager, err := database.NewTransactionManager(db)
	if err != nil {
		return nil, ErrInvalidLocaleRepository
	}
	return NewLocaleRepositoryWithDependencies(db, manager, manager)
}

// NewLocaleRepositoryWithDependencies lets application composition share one
// transaction boundary with audit and other system mutations.
func NewLocaleRepositoryWithDependencies(
	db *gormdb.DB,
	connections database.ConnectionProvider,
	transactions database.Transactor,
) (*LocaleRepository, error) {
	if db == nil || connections == nil || transactions == nil {
		return nil, ErrInvalidLocaleRepository
	}
	return &LocaleRepository{
		db: db, connections: connections, transactions: transactions,
	}, nil
}

func (repository *LocaleRepository) connection(ctx context.Context) (*gormdb.DB, error) {
	if repository == nil || ctx == nil || repository.connections == nil {
		return nil, ErrInvalidLocaleRepository
	}
	return repository.connections.Connection(ctx)
}

func (repository *LocaleRepository) withinTransaction(
	ctx context.Context,
	operation func(context.Context) error,
) error {
	if repository == nil || repository.transactions == nil || ctx == nil || operation == nil {
		return ErrInvalidLocaleRepository
	}
	return repository.transactions.WithinTransaction(ctx, operation)
}

func (repository *LocaleRepository) ListEnabled(
	ctx context.Context,
) ([]localizationdomain.Locale, error) {
	var records []localeRecord
	db, err := repository.connection(ctx)
	if err != nil {
		return nil, err
	}
	err = db.
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
	db, err := repository.connection(ctx)
	if err != nil {
		return nil, err
	}
	err = db.
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
