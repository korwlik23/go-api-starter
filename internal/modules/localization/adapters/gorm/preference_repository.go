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
	"gorm.io/gorm/clause"
)

var ErrInvalidPreferenceRepository = errors.New(
	"invalid locale preference repository",
)

type PreferenceRepository struct {
	db  *gormdb.DB
	now func() time.Time
}

type preferenceRecord struct {
	UserID       databaseid.ID `gorm:"column:user_id"`
	AllLanguages bool          `gorm:"column:all_languages"`
	Version      int64         `gorm:"column:version"`
}

func (preferenceRecord) TableName() string {
	return "user_locale_preferences"
}

type preferenceLocaleRecord struct {
	LocaleID databaseid.ID `gorm:"column:locale_id"`
}

func NewPreferenceRepository(
	db *gormdb.DB,
	now func() time.Time,
) (*PreferenceRepository, error) {
	if db == nil || now == nil {
		return nil, ErrInvalidPreferenceRepository
	}
	return &PreferenceRepository{db: db, now: now}, nil
}

func (repository *PreferenceRepository) Load(
	ctx context.Context,
	userID string,
) (localizationdomain.LocalePreference, error) {
	encodedUserID, err := databaseid.Encode(
		repository.db.Dialector.Name(),
		userID,
	)
	if err != nil {
		return localizationdomain.LocalePreference{},
			ports.ErrLocalePreferenceNotFound
	}
	var header preferenceRecord
	err = repository.db.WithContext(ctx).
		Where("user_id = ?", encodedUserID).
		First(&header).
		Error
	if errors.Is(err, gormdb.ErrRecordNotFound) {
		return localizationdomain.LocalePreference{},
			ports.ErrLocalePreferenceNotFound
	}
	if err != nil {
		return localizationdomain.LocalePreference{}, fmt.Errorf(
			"load locale preference: %w",
			err,
		)
	}
	result := localizationdomain.LocalePreference{
		AllLanguages: header.AllLanguages,
		Version:      header.Version,
	}
	if header.AllLanguages {
		return result, nil
	}
	var selected []preferenceLocaleRecord
	err = repository.db.WithContext(ctx).
		Table("user_locale_preference_locales").
		Select("locales.id AS locale_id").
		Joins(
			"JOIN locales ON locales.id = "+
				"user_locale_preference_locales.locale_id",
		).
		Where(
			"user_locale_preference_locales.user_id = ? "+
				"AND locales.enabled = ? AND locales.user_selectable = ?",
			encodedUserID,
			true,
			true,
		).
		Order("locales.tag ASC").
		Find(&selected).
		Error
	if err != nil {
		return localizationdomain.LocalePreference{}, fmt.Errorf(
			"load selected locales: %w",
			err,
		)
	}
	for _, record := range selected {
		result.LocaleIDs = append(result.LocaleIDs, record.LocaleID.String())
	}
	if len(result.LocaleIDs) == 0 {
		var fallback localeRecord
		err = repository.db.WithContext(ctx).
			Where("default_slot = ? AND enabled = ?", int16(1), true).
			First(&fallback).
			Error
		if err != nil {
			return localizationdomain.LocalePreference{},
				ErrInvalidPreferenceRepository
		}
		result.LocaleIDs = []string{fallback.ID.String()}
	}
	return result, nil
}

func (repository *PreferenceRepository) Save(
	ctx context.Context,
	userID string,
	expectedVersion int64,
	preference localizationdomain.LocalePreference,
) (localizationdomain.LocalePreference, error) {
	if expectedVersion < 0 ||
		(preference.AllLanguages && len(preference.LocaleIDs) != 0) ||
		(!preference.AllLanguages && len(preference.LocaleIDs) == 0) {
		return localizationdomain.LocalePreference{},
			ErrInvalidPreferenceRepository
	}
	encodedUserID, err := databaseid.Encode(
		repository.db.Dialector.Name(),
		userID,
	)
	if err != nil {
		return localizationdomain.LocalePreference{},
			ErrInvalidPreferenceRepository
	}
	encodedLocaleIDs := make([]any, 0, len(preference.LocaleIDs))
	for _, localeID := range preference.LocaleIDs {
		encoded, encodeErr := databaseid.Encode(
			repository.db.Dialector.Name(),
			localeID,
		)
		if encodeErr != nil {
			return localizationdomain.LocalePreference{},
				ErrInvalidPreferenceRepository
		}
		encodedLocaleIDs = append(encodedLocaleIDs, encoded)
	}
	now := repository.now()
	newVersion := expectedVersion + 1
	err = repository.db.WithContext(ctx).Transaction(func(tx *gormdb.DB) error {
		var header preferenceRecord
		findErr := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("user_id = ?", encodedUserID).
			First(&header).
			Error
		switch {
		case errors.Is(findErr, gormdb.ErrRecordNotFound) &&
			expectedVersion == 0:
			if createErr := tx.Table("user_locale_preferences").
				Create(map[string]any{
					"user_id":       encodedUserID,
					"all_languages": preference.AllLanguages,
					"version":       int64(1),
					"created_at":    now,
					"updated_at":    now,
				}).Error; createErr != nil {
				return ports.ErrLocalePreferenceConflict
			}
		case findErr != nil:
			return ports.ErrLocalePreferenceConflict
		case header.Version != expectedVersion || expectedVersion == 0:
			return ports.ErrLocalePreferenceConflict
		default:
			result := tx.Model(&preferenceRecord{}).
				Where(
					"user_id = ? AND version = ?",
					encodedUserID,
					expectedVersion,
				).
				Updates(map[string]any{
					"all_languages": preference.AllLanguages,
					"version":       newVersion,
					"updated_at":    now,
				})
			if result.Error != nil || result.RowsAffected != 1 {
				return ports.ErrLocalePreferenceConflict
			}
		}

		if !preference.AllLanguages {
			var eligible []localeRecord
			if lockErr := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
				Select("id").
				Where(
					"id IN ? AND enabled = ? AND user_selectable = ?",
					encodedLocaleIDs,
					true,
					true,
				).
				Find(&eligible).
				Error; lockErr != nil || len(eligible) != len(encodedLocaleIDs) {
				return ErrInvalidPreferenceRepository
			}
		}
		if deleteErr := tx.Exec(
			"DELETE FROM user_locale_preference_locales WHERE user_id = ?",
			encodedUserID,
		).Error; deleteErr != nil {
			return deleteErr
		}
		if preference.AllLanguages {
			return nil
		}
		rows := make([]map[string]any, 0, len(encodedLocaleIDs))
		for _, localeID := range encodedLocaleIDs {
			rows = append(rows, map[string]any{
				"user_id":   encodedUserID,
				"locale_id": localeID,
			})
		}
		return tx.Table("user_locale_preference_locales").
			Create(&rows).
			Error
	})
	if err != nil {
		return localizationdomain.LocalePreference{}, fmt.Errorf(
			"save locale preference: %w",
			err,
		)
	}
	preference.Version = newVersion
	preference.LocaleIDs = append([]string(nil), preference.LocaleIDs...)
	return preference, nil
}

var _ ports.PreferenceRepository = (*PreferenceRepository)(nil)
