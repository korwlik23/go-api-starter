package gorm

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/korwlik23/go-api-starter/internal/modules/publishing/ports"
	"github.com/korwlik23/go-api-starter/internal/platform/databaseid"
	gormdb "gorm.io/gorm"
)

func (repository *ContentRepository) Create(
	ctx context.Context,
	command ports.CreateContentCommand,
) (ports.ContentDetail, error) {
	db, err := repository.connection(ctx)
	if err != nil {
		return ports.ContentDetail{}, err
	}
	contentID, err := databaseid.Encode(db.Dialector.Name(), command.Item.ID)
	if err != nil {
		return ports.ContentDetail{}, ports.ErrInvalidContentRepository
	}
	accountID, err := databaseid.Encode(db.Dialector.Name(), command.Item.AccountID)
	if err != nil {
		return ports.ContentDetail{}, ports.ErrInvalidContentRepository
	}
	translationID, err := databaseid.Encode(db.Dialector.Name(), command.Translation.ID)
	if err != nil {
		return ports.ContentDetail{}, ports.ErrInvalidContentRepository
	}
	localeID, localeCanonical, err := repository.resolveLocaleID(ctx, command.Translation.Locale)
	if err != nil {
		return ports.ContentDetail{}, err
	}
	blocks, err := json.Marshal(command.Translation.Blocks)
	if err != nil {
		return ports.ContentDetail{}, ports.ErrInvalidContentRepository
	}
	if createErr := db.Table("content_items").Create(map[string]any{
		"id":          contentID,
		"account_id":  accountID,
		"kind":        command.Item.Kind,
		"content_key": command.Item.ContentKey,
		"status":      command.Item.Status,
		"version":     command.Item.Version,
		"created_at":  command.Item.CreatedAt,
		"updated_at":  command.Item.UpdatedAt,
	}).Error; createErr != nil {
		return ports.ContentDetail{}, fmt.Errorf("create content item: %w", createErr)
	}
	if createErr := db.Table("content_translations").Create(map[string]any{
		"id":              translationID,
		"account_id":      accountID,
		"content_id":      contentID,
		"locale_id":       localeID,
		"slug":            command.Translation.Slug,
		"path":            command.Translation.Path,
		"title":           command.Translation.Title,
		"excerpt":         nullableString(command.Translation.Excerpt),
		"blocks":          blocks,
		"seo_title":       nullableString(command.Translation.SEO.Title),
		"seo_description": nullableString(command.Translation.SEO.Description),
		"geo_region":      nullableString(command.Translation.GEO.Region),
		"geo_locality":    nullableString(command.Translation.GEO.Locality),
		"aeo_question":    nullableString(command.Translation.AEO.Question),
		"aeo_answer":      nullableString(command.Translation.AEO.Answer),
		"status":          command.Translation.Status,
		"version":         command.Translation.Version,
		"created_at":      command.Translation.CreatedAt,
		"updated_at":      command.Translation.UpdatedAt,
	}).Error; createErr != nil {
		return ports.ContentDetail{}, fmt.Errorf("create content translation: %w", createErr)
	}
	return repository.Get(ctx, command.Item.AccountID, command.Item.ID, localeCanonical)
}

func (repository *ContentRepository) Update(
	ctx context.Context,
	command ports.UpdateContentCommand,
) (ports.ContentDetail, error) {
	db, err := repository.connection(ctx)
	if err != nil {
		return ports.ContentDetail{}, err
	}
	accountID, err := databaseid.Encode(db.Dialector.Name(), command.AccountID)
	if err != nil {
		return ports.ContentDetail{}, ports.ErrContentNotFound
	}
	contentID, err := databaseid.Encode(db.Dialector.Name(), command.ContentID)
	if err != nil {
		return ports.ContentDetail{}, ports.ErrContentNotFound
	}
	translationID, err := databaseid.Encode(db.Dialector.Name(), command.TranslationID)
	if err != nil {
		return ports.ContentDetail{}, ports.ErrContentNotFound
	}
	localeValue := command.LocaleID
	if localeValue == "" {
		localeValue = command.Translation.Locale
	}
	localeID, localeCanonical, err := repository.resolveLocaleID(ctx, localeValue)
	if err != nil {
		return ports.ContentDetail{}, err
	}
	blocks, err := json.Marshal(command.Translation.Blocks)
	if err != nil {
		return ports.ContentDetail{}, ports.ErrInvalidContentRepository
	}
	result := db.Table("content_translations").
		Where(
			"account_id = ? AND content_id = ? AND id = ? AND version = ? "+
				"AND deleted_at IS NULL",
			accountID, contentID, translationID, command.ExpectedVersion,
		).
		Updates(map[string]any{
			"locale_id":       localeID,
			"slug":            command.Translation.Slug,
			"path":            command.Translation.Path,
			"title":           command.Translation.Title,
			"excerpt":         nullableString(command.Translation.Excerpt),
			"blocks":          blocks,
			"seo_title":       nullableString(command.Translation.SEO.Title),
			"seo_description": nullableString(command.Translation.SEO.Description),
			"geo_region":      nullableString(command.Translation.GEO.Region),
			"geo_locality":    nullableString(command.Translation.GEO.Locality),
			"aeo_question":    nullableString(command.Translation.AEO.Question),
			"aeo_answer":      nullableString(command.Translation.AEO.Answer),
			"status":          command.Translation.Status,
			"version":         command.ExpectedVersion + 1,
			"updated_at":      command.Translation.UpdatedAt,
		})
	if result.Error != nil {
		return ports.ContentDetail{}, fmt.Errorf("update content translation: %w", result.Error)
	}
	if result.RowsAffected != 1 {
		return ports.ContentDetail{}, ports.ErrContentConflict
	}
	itemResult := db.Table("content_items").
		Where("account_id = ? AND id = ? AND deleted_at IS NULL", accountID, contentID).
		Updates(map[string]any{
			"version":    gormdb.Expr("version + ?", 1),
			"updated_at": command.Translation.UpdatedAt,
		})
	if itemResult.Error != nil {
		return ports.ContentDetail{}, fmt.Errorf("update content item: %w", itemResult.Error)
	}
	if itemResult.RowsAffected != 1 {
		return ports.ContentDetail{}, ports.ErrContentNotFound
	}
	return repository.Get(ctx, command.AccountID, command.ContentID, localeCanonical)
}

func (repository *ContentRepository) SoftDelete(
	ctx context.Context,
	command ports.DeleteContentCommand,
) error {
	db, err := repository.connection(ctx)
	if err != nil {
		return err
	}
	accountID, err := databaseid.Encode(db.Dialector.Name(), command.AccountID)
	if err != nil {
		return ports.ErrContentNotFound
	}
	contentID, err := databaseid.Encode(db.Dialector.Name(), command.ContentID)
	if err != nil {
		return ports.ErrContentNotFound
	}
	translationID, err := databaseid.Encode(db.Dialector.Name(), command.TranslationID)
	if err != nil {
		return ports.ErrContentNotFound
	}
	translationResult := db.Table("content_translations").
		Where(
			"account_id = ? AND content_id = ? AND id = ? AND version = ? "+
				"AND deleted_at IS NULL",
			accountID, contentID, translationID, command.ExpectedVersion,
		).
		Updates(map[string]any{
			"deleted_at": command.DeletedAt,
			"version":    command.ExpectedVersion + 1,
			"updated_at": command.DeletedAt,
		})
	if translationResult.Error != nil {
		return fmt.Errorf("delete content translation: %w", translationResult.Error)
	}
	if translationResult.RowsAffected != 1 {
		return ports.ErrContentConflict
	}
	itemResult := db.Table("content_items").
		Where("account_id = ? AND id = ? AND deleted_at IS NULL", accountID, contentID).
		Updates(map[string]any{
			"deleted_at": command.DeletedAt,
			"version":    gormdb.Expr("version + ?", 1),
			"updated_at": command.DeletedAt,
		})
	if itemResult.Error != nil {
		return fmt.Errorf("delete content item: %w", itemResult.Error)
	}
	if itemResult.RowsAffected != 1 {
		return ports.ErrContentNotFound
	}
	return nil
}

func (repository *ContentRepository) resolveLocaleID(
	ctx context.Context,
	value string,
) (any, string, error) {
	if encoded, err := databaseid.Encode(repository.db.Dialector.Name(), value); err == nil {
		return encoded, value, nil
	}
	db, err := repository.connection(ctx)
	if err != nil {
		return nil, "", err
	}
	var localeRecord struct {
		ID databaseid.ID `gorm:"column:id"`
	}
	result := db.Table("locales").Select("id").Where("tag = ?", value).Limit(1).Scan(&localeRecord)
	if result.Error != nil {
		return nil, "", fmt.Errorf("resolve content locale: %w", result.Error)
	}
	if result.RowsAffected == 0 || localeRecord.ID.String() == "" {
		return nil, "", ports.ErrContentNotFound
	}
	encoded, err := databaseid.Encode(repository.db.Dialector.Name(), localeRecord.ID.String())
	if err != nil {
		return nil, "", err
	}
	return encoded, localeRecord.ID.String(), nil
}

func nullableString(value string) any {
	if value == "" {
		return nil
	}
	return value
}
