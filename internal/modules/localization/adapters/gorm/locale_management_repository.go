package gorm

import (
	"context"
	"fmt"
	"strings"
	"time"

	localizationdomain "github.com/korwlik23/go-api-starter/internal/modules/localization/domain"
	"github.com/korwlik23/go-api-starter/internal/modules/localization/ports"
	"github.com/korwlik23/go-api-starter/internal/platform/databaseid"
	gormdb "gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const maximumFallbackDepth = 32

func (repository *LocaleRepository) FindByID(
	ctx context.Context,
	localeID string,
) (localizationdomain.Locale, error) {
	encodedID, err := databaseid.Encode(
		repository.db.Dialector.Name(),
		localeID,
	)
	if err != nil {
		return localizationdomain.Locale{}, ports.ErrLocaleNotFound
	}
	var record localeRecord
	err = repository.db.WithContext(ctx).
		Where("id = ?", encodedID).
		First(&record).
		Error
	if err == gormdb.ErrRecordNotFound {
		return localizationdomain.Locale{}, ports.ErrLocaleNotFound
	}
	if err != nil {
		return localizationdomain.Locale{}, fmt.Errorf("find locale: %w", err)
	}
	return localeFromRecord(record)
}

func (repository *LocaleRepository) Create(
	ctx context.Context,
	command ports.CreateLocaleCommand,
) error {
	locale := command.Locale
	tag, err := localizationdomain.NormalizeLocaleTag(locale.Tag)
	if err != nil ||
		tag != locale.Tag ||
		locale.ID == "" ||
		strings.TrimSpace(locale.Name) == "" ||
		locale.Name != strings.TrimSpace(locale.Name) ||
		!validDirection(locale.Direction) ||
		locale.Version != 1 ||
		locale.Default ||
		locale.FallbackID != "" ||
		locale.CreatedAt.IsZero() ||
		locale.UpdatedAt != locale.CreatedAt ||
		(locale.UserSelectable && !locale.Enabled) {
		return ErrInvalidLocaleRepository
	}
	encodedID, err := databaseid.Encode(
		repository.db.Dialector.Name(),
		locale.ID,
	)
	if err != nil {
		return ErrInvalidLocaleRepository
	}
	err = repository.db.WithContext(ctx).
		Table("locales").
		Create(map[string]any{
			"id":                 encodedID,
			"tag":                locale.Tag,
			"name":               locale.Name,
			"direction":          string(locale.Direction),
			"fallback_locale_id": nil,
			"enabled":            locale.Enabled,
			"user_selectable":    locale.UserSelectable,
			"default_slot":       nil,
			"version":            int64(1),
			"created_at":         locale.CreatedAt,
			"updated_at":         locale.UpdatedAt,
		}).
		Error
	if err != nil {
		return fmt.Errorf("%w: %v", ports.ErrLocaleConflict, err)
	}
	return nil
}

func (repository *LocaleRepository) Update(
	ctx context.Context,
	command ports.UpdateLocaleCommand,
) (int64, error) {
	if command.ExpectedVersion < 1 ||
		strings.TrimSpace(command.Name) == "" ||
		command.Name != strings.TrimSpace(command.Name) ||
		!validDirection(command.Direction) ||
		command.UpdatedAt.IsZero() ||
		(command.UserSelectable && !command.Enabled) {
		return 0, ErrInvalidLocaleRepository
	}
	encodedID, err := databaseid.Encode(
		repository.db.Dialector.Name(),
		command.ID,
	)
	if err != nil {
		return 0, ports.ErrLocaleNotFound
	}
	var encodedFallbackID any
	if command.FallbackID != "" {
		encodedFallbackID, err = databaseid.Encode(
			repository.db.Dialector.Name(),
			command.FallbackID,
		)
		if err != nil || command.FallbackID == command.ID {
			return 0, ErrInvalidLocaleRepository
		}
	}

	newVersion := command.ExpectedVersion + 1
	err = repository.db.WithContext(ctx).Transaction(func(tx *gormdb.DB) error {
		var records []localeRecord
		if lockErr := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Order("id ASC").
			Find(&records).
			Error; lockErr != nil {
			return lockErr
		}
		if validationErr := validateLocaleUpdate(
			records,
			command,
		); validationErr != nil {
			return validationErr
		}
		result := tx.Model(&localeRecord{}).
			Where("id = ? AND version = ?", encodedID, command.ExpectedVersion).
			Updates(map[string]any{
				"name":               command.Name,
				"direction":          string(command.Direction),
				"fallback_locale_id": encodedFallbackID,
				"enabled":            command.Enabled,
				"user_selectable":    command.UserSelectable,
				"version":            newVersion,
				"updated_at":         command.UpdatedAt,
			})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return ports.ErrLocaleConflict
		}
		return nil
	})
	if err != nil {
		return 0, fmt.Errorf("update locale: %w", err)
	}
	return newVersion, nil
}

func validateLocaleUpdate(
	records []localeRecord,
	command ports.UpdateLocaleCommand,
) error {
	locales := make(map[string]localeRecord, len(records))
	for _, record := range records {
		locales[record.ID.String()] = record
	}
	target, ok := locales[command.ID]
	if !ok {
		return ports.ErrLocaleNotFound
	}
	if target.Version != command.ExpectedVersion {
		return ports.ErrLocaleConflict
	}
	if target.DefaultSlot != nil &&
		(!command.Enabled || command.FallbackID != "") {
		return ports.ErrLocaleConflict
	}
	if command.FallbackID != "" {
		fallback, exists := locales[command.FallbackID]
		if !exists || !fallback.Enabled {
			return ports.ErrLocaleConflict
		}
	}

	current := command.FallbackID
	visited := map[string]struct{}{command.ID: {}}
	for depth := 0; current != ""; depth++ {
		if depth >= maximumFallbackDepth {
			return ports.ErrLocaleConflict
		}
		if _, cycle := visited[current]; cycle {
			return ports.ErrLocaleConflict
		}
		visited[current] = struct{}{}
		record, exists := locales[current]
		if !exists || !record.Enabled {
			return ports.ErrLocaleConflict
		}
		current = ""
		if record.FallbackLocaleID != nil {
			current = record.FallbackLocaleID.String()
		}
	}
	return nil
}

func (repository *LocaleRepository) SetDefault(
	ctx context.Context,
	localeID string,
	expectedVersion int64,
	updatedAt time.Time,
) (ports.DefaultLocaleResult, error) {
	if expectedVersion < 1 || updatedAt.IsZero() {
		return ports.DefaultLocaleResult{}, ErrInvalidLocaleRepository
	}
	encodedID, err := databaseid.Encode(
		repository.db.Dialector.Name(),
		localeID,
	)
	if err != nil {
		return ports.DefaultLocaleResult{}, ports.ErrLocaleNotFound
	}
	result := ports.DefaultLocaleResult{Version: expectedVersion}
	err = repository.db.WithContext(ctx).Transaction(func(tx *gormdb.DB) error {
		var records []localeRecord
		if lockErr := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("default_slot = ? OR id = ?", int16(1), encodedID).
			Order("id ASC").
			Find(&records).
			Error; lockErr != nil {
			return lockErr
		}
		var target *localeRecord
		var current *localeRecord
		for index := range records {
			if records[index].ID.String() == localeID {
				target = &records[index]
			}
			if records[index].DefaultSlot != nil {
				current = &records[index]
			}
		}
		if target == nil ||
			target.Version != expectedVersion ||
			!target.Enabled ||
			target.FallbackLocaleID != nil {
			return ports.ErrLocaleConflict
		}
		if current != nil && current.ID == target.ID {
			result.PreviousDefaultLocaleID = current.ID.String()
			return nil
		}
		if current != nil {
			result.PreviousDefaultLocaleID = current.ID.String()
			encodedCurrentID, encodeErr := databaseid.Encode(
				repository.db.Dialector.Name(),
				current.ID.String(),
			)
			if encodeErr != nil {
				return ports.ErrLocaleConflict
			}
			result := tx.Model(&localeRecord{}).
				Where("id = ? AND version = ?", encodedCurrentID, current.Version).
				Updates(map[string]any{
					"default_slot": nil,
					"version":      current.Version + 1,
					"updated_at":   updatedAt,
				})
			if result.Error != nil || result.RowsAffected != 1 {
				return ports.ErrLocaleConflict
			}
		}
		updateResult := tx.Model(&localeRecord{}).
			Where("id = ? AND version = ?", encodedID, expectedVersion).
			Updates(map[string]any{
				"default_slot": int16(1),
				"version":      expectedVersion + 1,
				"updated_at":   updatedAt,
			})
		if updateResult.Error != nil {
			return updateResult.Error
		}
		if updateResult.RowsAffected != 1 {
			return ports.ErrLocaleConflict
		}
		result.Version = expectedVersion + 1
		return nil
	})
	if err != nil {
		return ports.DefaultLocaleResult{}, fmt.Errorf("set default locale: %w", err)
	}
	return result, nil
}

func validDirection(direction localizationdomain.Direction) bool {
	return direction == localizationdomain.DirectionLTR ||
		direction == localizationdomain.DirectionRTL
}

var _ ports.LocaleManagementRepository = (*LocaleRepository)(nil)
