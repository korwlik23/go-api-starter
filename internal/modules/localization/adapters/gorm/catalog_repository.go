package gorm

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	localizationdomain "github.com/korwlik23/go-api-starter/internal/modules/localization/domain"
	"github.com/korwlik23/go-api-starter/internal/modules/localization/ports"
	"github.com/korwlik23/go-api-starter/internal/platform/databaseid"
	gormdb "gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var ErrInvalidCatalogRepository = errors.New(
	"invalid catalog repository",
)

var (
	categoryPattern = regexp.MustCompile(
		`^[a-z][a-z0-9]*([._-][a-z0-9]+)*$`,
	)
	translationKeyPattern = regexp.MustCompile(
		`^[a-z][a-z0-9]*([_-][a-z0-9]+)*(\.[a-z][a-z0-9]*([_-][a-z0-9]+)*)*$`,
	)
)

type CatalogRepository struct {
	db  *gormdb.DB
	now func() time.Time
}

type translationRecord struct {
	LocaleID       databaseid.ID `gorm:"column:locale_id"`
	Category       string        `gorm:"column:category"`
	TranslationKey string        `gorm:"column:translation_key"`
	Value          *string       `gorm:"column:value"`
	Source         string        `gorm:"column:source"`
	Version        int64         `gorm:"column:version"`
}

func (translationRecord) TableName() string {
	return "translations"
}

func NewCatalogRepository(
	db *gormdb.DB,
	now func() time.Time,
) (*CatalogRepository, error) {
	if db == nil || now == nil {
		return nil, ErrInvalidCatalogRepository
	}
	return &CatalogRepository{db: db, now: now}, nil
}

func (repository *CatalogRepository) FindEnabledLocaleByTag(
	ctx context.Context,
	tag string,
) (localizationdomain.Locale, error) {
	var record localeRecord
	err := repository.db.WithContext(ctx).
		Where("tag = ? AND enabled = ?", tag, true).
		First(&record).
		Error
	return repository.mapFoundLocale(record, err)
}

func (repository *CatalogRepository) FindEnabledLocaleByID(
	ctx context.Context,
	id string,
) (localizationdomain.Locale, error) {
	encodedID, err := databaseid.Encode(
		repository.db.Dialector.Name(),
		id,
	)
	if err != nil {
		return localizationdomain.Locale{}, ports.ErrCatalogNotFound
	}
	var record localeRecord
	err = repository.db.WithContext(ctx).
		Where("id = ? AND enabled = ?", encodedID, true).
		First(&record).
		Error
	return repository.mapFoundLocale(record, err)
}

func (repository *CatalogRepository) mapFoundLocale(
	record localeRecord,
	err error,
) (localizationdomain.Locale, error) {
	if errors.Is(err, gormdb.ErrRecordNotFound) {
		return localizationdomain.Locale{}, ports.ErrCatalogNotFound
	}
	if err != nil {
		return localizationdomain.Locale{}, fmt.Errorf("find catalog locale: %w", err)
	}
	return localeFromRecord(record)
}

func (repository *CatalogRepository) ListActiveOverrides(
	ctx context.Context,
	localeID string,
	category string,
) ([]ports.CatalogOverride, error) {
	if !validCatalogCategory(category) {
		return nil, ErrInvalidCatalogRepository
	}
	encodedID, err := databaseid.Encode(
		repository.db.Dialector.Name(),
		localeID,
	)
	if err != nil {
		return nil, ports.ErrCatalogNotFound
	}
	var records []translationRecord
	err = repository.db.WithContext(ctx).
		Where(
			"locale_id = ? AND category = ? AND value IS NOT NULL",
			encodedID,
			category,
		).
		Order("translation_key ASC").
		Find(&records).
		Error
	if err != nil {
		return nil, fmt.Errorf("list catalog overrides: %w", err)
	}
	overrides := make([]ports.CatalogOverride, 0, len(records))
	for _, record := range records {
		if record.Value == nil ||
			!validTranslationKey(record.TranslationKey) ||
			record.Version < 1 {
			return nil, ErrInvalidCatalogRepository
		}
		overrides = append(overrides, ports.CatalogOverride{
			Key: record.TranslationKey, Value: *record.Value, Version: record.Version,
		})
	}
	return overrides, nil
}

func (repository *CatalogRepository) ReplaceCategory(
	ctx context.Context,
	localeID string,
	category string,
	entries map[string]string,
	expectedVersion int64,
	source string,
) (int64, error) {
	if !validCatalogCategory(category) ||
		expectedVersion < 0 ||
		(expectedVersion == 0 && len(entries) == 0) ||
		(source != "editor" && source != "import") {
		return 0, ErrInvalidCatalogRepository
	}
	for key, value := range entries {
		if !validTranslationKey(key) ||
			len(value) < 1 ||
			len(value) > 16_384 {
			return 0, ErrInvalidCatalogRepository
		}
	}
	encodedID, err := databaseid.Encode(
		repository.db.Dialector.Name(),
		localeID,
	)
	if err != nil {
		return 0, ports.ErrCatalogNotFound
	}
	newVersion := expectedVersion + 1
	now := repository.now()
	err = repository.db.WithContext(ctx).Transaction(func(tx *gormdb.DB) error {
		var locale localeRecord
		if lockErr := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ? AND enabled = ?", encodedID, true).
			First(&locale).
			Error; lockErr != nil {
			return ports.ErrCatalogNotFound
		}
		var existing []translationRecord
		if lockErr := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("locale_id = ? AND category = ?", encodedID, category).
			Order("translation_key ASC").
			Find(&existing).
			Error; lockErr != nil {
			return lockErr
		}
		currentVersion := int64(0)
		for _, record := range existing {
			if record.Version > currentVersion {
				currentVersion = record.Version
			}
		}
		if currentVersion != expectedVersion {
			return ports.ErrCatalogConflict
		}
		existingByKey := make(map[string]translationRecord, len(existing))
		for _, record := range existing {
			existingByKey[record.TranslationKey] = record
			value, present := entries[record.TranslationKey]
			var storedValue any
			if present {
				storedValue = value
			}
			result := tx.Model(&translationRecord{}).
				Where(
					"locale_id = ? AND category = ? AND translation_key = ? "+
						"AND version = ?",
					encodedID,
					category,
					record.TranslationKey,
					record.Version,
				).
				Updates(map[string]any{
					"value":      storedValue,
					"source":     source,
					"version":    newVersion,
					"updated_at": now,
				})
			if result.Error != nil || result.RowsAffected != 1 {
				return ports.ErrCatalogConflict
			}
		}
		for key, value := range entries {
			if _, exists := existingByKey[key]; exists {
				continue
			}
			if createErr := tx.Table("translations").
				Create(map[string]any{
					"locale_id":       encodedID,
					"category":        category,
					"translation_key": key,
					"value":           value,
					"source":          source,
					"version":         newVersion,
					"created_at":      now,
					"updated_at":      now,
				}).Error; createErr != nil {
				return ports.ErrCatalogConflict
			}
		}
		return nil
	})
	if err != nil {
		return 0, fmt.Errorf("replace translation category: %w", err)
	}
	return newVersion, nil
}

func validCatalogCategory(value string) bool {
	return len(value) >= 1 &&
		len(value) <= 64 &&
		categoryPattern.MatchString(value) &&
		!containsPrototypeSegment(value)
}

func validTranslationKey(value string) bool {
	return len(value) >= 1 &&
		len(value) <= 255 &&
		translationKeyPattern.MatchString(value) &&
		!containsPrototypeSegment(value)
}

func containsPrototypeSegment(value string) bool {
	for _, segment := range strings.FieldsFunc(
		value,
		func(character rune) bool {
			return character == '.' || character == '_' || character == '-'
		},
	) {
		if segment == "prototype" || segment == "constructor" {
			return true
		}
	}
	return false
}

var _ ports.CatalogRepository = (*CatalogRepository)(nil)
var _ ports.CatalogUpdateRepository = (*CatalogRepository)(nil)
