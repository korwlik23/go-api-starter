package gorm

import (
	"context"
	"fmt"
	"strings"

	publishingdomain "github.com/korwlik23/go-api-starter/internal/modules/publishing/domain"
	"github.com/korwlik23/go-api-starter/internal/modules/publishing/ports"
	"github.com/korwlik23/go-api-starter/internal/platform/databaseid"
	gormdb "gorm.io/gorm"
)

func (repository *ContentRepository) List(ctx context.Context, query ports.ContentListQuery) (ports.ContentListPage, error) {
	if repository == nil || repository.db == nil || ctx == nil {
		return ports.ContentListPage{}, ErrInvalidContentRepository
	}
	if err := ports.ValidateContentListQuery(query); err != nil {
		return ports.ContentListPage{}, err
	}
	accountID, err := databaseid.Encode(repository.db.Dialector.Name(), query.AccountID)
	if err != nil {
		return ports.ContentListPage{}, ports.ErrInvalidContentQuery
	}
	localeID, err := databaseid.Encode(repository.db.Dialector.Name(), query.LocaleID)
	if err != nil {
		return ports.ContentListPage{}, ports.ErrInvalidContentQuery
	}
	db, err := repository.connection(ctx)
	if err != nil {
		return ports.ContentListPage{}, err
	}
	db = db.WithContext(ctx).Table("content_translations").Select(contentListSelect).Joins(contentJoin).
		Where("content_items.account_id = ? AND content_translations.locale_id = ? AND content_items.deleted_at IS NULL AND content_translations.deleted_at IS NULL", accountID, localeID)
	if query.Kind != nil {
		db = db.Where("content_items.kind = ?", string(*query.Kind))
	}
	if len(query.Status) > 0 {
		statuses := make([]string, 0, len(query.Status))
		for _, status := range query.Status {
			statuses = append(statuses, string(status))
		}
		db = db.Where("content_translations.status IN ?", statuses)
	}
	if query.After != nil {
		cursorID, encodeErr := databaseid.Encode(repository.db.Dialector.Name(), query.After.ID)
		if encodeErr != nil {
			return ports.ContentListPage{}, ports.ErrInvalidContentQuery
		}
		db = db.Where("(content_translations.updated_at < ? OR (content_translations.updated_at = ? AND content_translations.id < ?))", query.After.UpdatedAt, query.After.UpdatedAt, cursorID)
	}
	var records []contentListRecord
	if err := db.Order("content_translations.updated_at DESC, content_translations.id DESC").Limit(query.Limit + 1).Scan(&records).Error; err != nil {
		return ports.ContentListPage{}, fmt.Errorf("list content: %w", err)
	}
	return listPage(records, query.Limit)
}

func (repository *ContentRepository) Get(ctx context.Context, accountID, contentID, localeID string) (ports.ContentDetail, error) {
	if repository == nil || repository.db == nil || ctx == nil {
		return ports.ContentDetail{}, ErrInvalidContentRepository
	}
	encodedAccountID, encodedContentID, encodedLocaleID, err := repository.encodeIDs(accountID, contentID, localeID)
	if err != nil {
		return ports.ContentDetail{}, ports.ErrContentNotFound
	}
	var record contentDetailRecord
	db, err := repository.detailQuery(ctx)
	if err != nil {
		return ports.ContentDetail{}, err
	}
	result := db.Where("content_items.account_id = ? AND content_translations.content_id = ? AND content_translations.locale_id = ? AND content_items.deleted_at IS NULL AND content_translations.deleted_at IS NULL", encodedAccountID, encodedContentID, encodedLocaleID).Limit(1).Scan(&record)
	if result.Error != nil {
		return ports.ContentDetail{}, fmt.Errorf("get content: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return ports.ContentDetail{}, ports.ErrContentNotFound
	}
	return mapContentDetailRecord(record)
}

func (repository *ContentRepository) ListPublishedAlternates(ctx context.Context, accountID, contentID, excludeLocaleID string) ([]ports.ContentDetail, error) {
	if repository == nil || repository.db == nil || ctx == nil {
		return nil, ErrInvalidContentRepository
	}
	if err := ports.ValidateContentListQuery(ports.ContentListQuery{AccountID: accountID, LocaleID: excludeLocaleID, Limit: 1}); err != nil || strings.TrimSpace(contentID) == "" {
		return nil, ports.ErrInvalidContentQuery
	}
	encodedAccountID, encodedContentID, encodedLocaleID, err := repository.encodeIDs(accountID, contentID, excludeLocaleID)
	if err != nil {
		return nil, ports.ErrInvalidContentQuery
	}
	db, err := repository.detailQuery(ctx)
	if err != nil {
		return nil, err
	}
	var records []contentDetailRecord
	result := db.Where("content_items.account_id = ? AND content_translations.content_id = ? AND content_translations.locale_id <> ? AND content_items.status = ? AND content_translations.status = ? AND content_items.deleted_at IS NULL AND content_translations.deleted_at IS NULL", encodedAccountID, encodedContentID, encodedLocaleID, string(publishingdomain.StatusPublished), string(publishingdomain.StatusPublished)).Order("content_translations.locale_id ASC, content_translations.path ASC").Scan(&records)
	if result.Error != nil {
		return nil, fmt.Errorf("list public content alternates: %w", result.Error)
	}
	items := make([]ports.ContentDetail, 0, len(records))
	for _, record := range records {
		item, mapErr := mapContentDetailRecord(record)
		if mapErr != nil {
			return nil, mapErr
		}
		items = append(items, item)
	}
	return items, nil
}

func (repository *ContentRepository) ListPublishedPosts(ctx context.Context, accountID, localeID string, limit int, after *ports.ContentCursor) (ports.ContentListPage, error) {
	if repository == nil || repository.db == nil || ctx == nil {
		return ports.ContentListPage{}, ErrInvalidContentRepository
	}
	kind := publishingdomain.KindPost
	query := ports.ContentListQuery{AccountID: accountID, LocaleID: localeID, Kind: &kind, Limit: limit, After: after}
	if err := ports.ValidateContentListQuery(query); err != nil {
		return ports.ContentListPage{}, err
	}
	encodedAccountID, err := databaseid.Encode(repository.db.Dialector.Name(), accountID)
	if err != nil {
		return ports.ContentListPage{}, ports.ErrInvalidContentQuery
	}
	encodedLocaleID, err := databaseid.Encode(repository.db.Dialector.Name(), localeID)
	if err != nil {
		return ports.ContentListPage{}, ports.ErrInvalidContentQuery
	}
	db, err := repository.connection(ctx)
	if err != nil {
		return ports.ContentListPage{}, err
	}
	db = db.WithContext(ctx).Table("content_translations").Select(contentListSelect).Joins(contentJoin).
		Where("content_items.account_id = ? AND content_translations.locale_id = ? AND content_items.kind = ? AND content_items.status = ? AND content_translations.status = ? AND content_items.deleted_at IS NULL AND content_translations.deleted_at IS NULL", encodedAccountID, encodedLocaleID, string(publishingdomain.KindPost), string(publishingdomain.StatusPublished), string(publishingdomain.StatusPublished))
	if after != nil {
		cursorID, encodeErr := databaseid.Encode(repository.db.Dialector.Name(), after.ID)
		if encodeErr != nil {
			return ports.ContentListPage{}, ports.ErrInvalidContentQuery
		}
		db = db.Where("(content_translations.updated_at < ? OR (content_translations.updated_at = ? AND content_translations.id < ?))", after.UpdatedAt, after.UpdatedAt, cursorID)
	}
	var records []contentListRecord
	if err := db.Order("content_translations.updated_at DESC, content_translations.id DESC").Limit(limit + 1).Scan(&records).Error; err != nil {
		return ports.ContentListPage{}, fmt.Errorf("list public posts: %w", err)
	}
	return listPage(records, limit)
}

func (repository *ContentRepository) FindPublic(ctx context.Context, accountID, localeID, contentPath string) (ports.ContentDetail, error) {
	if repository == nil || repository.db == nil || ctx == nil {
		return ports.ContentDetail{}, ErrInvalidContentRepository
	}
	normalizedPath, err := publishingdomain.NormalizePath(contentPath)
	if err != nil {
		return ports.ContentDetail{}, ports.ErrContentNotFound
	}
	encodedAccountID, encodedLocaleID, err := repository.encodeAccountLocaleIDs(accountID, localeID)
	if err != nil {
		return ports.ContentDetail{}, ports.ErrContentNotFound
	}
	db, err := repository.detailQuery(ctx)
	if err != nil {
		return ports.ContentDetail{}, err
	}
	var record contentDetailRecord
	result := db.Where("content_items.account_id = ? AND content_translations.locale_id = ? AND content_translations.path = ? AND content_items.status = ? AND content_translations.status = ? AND content_items.deleted_at IS NULL AND content_translations.deleted_at IS NULL", encodedAccountID, encodedLocaleID, normalizedPath, string(publishingdomain.StatusPublished), string(publishingdomain.StatusPublished)).Limit(1).Scan(&record)
	if result.Error != nil {
		return ports.ContentDetail{}, fmt.Errorf("find public content: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return ports.ContentDetail{}, ports.ErrContentNotFound
	}
	return mapContentDetailRecord(record)
}

const contentListSelect = "content_items.id AS content_id, content_items.account_id, content_items.kind, content_items.content_key, content_items.status AS content_status, content_items.version AS content_version, content_translations.id AS translation_id, content_translations.locale_id, content_translations.slug, content_translations.path, content_translations.title, content_translations.status AS translation_status, content_translations.version AS translation_version, content_translations.updated_at"
const contentJoin = "JOIN content_items ON content_items.id = content_translations.content_id AND content_items.account_id = content_translations.account_id"

func (repository *ContentRepository) detailQuery(ctx context.Context) (*gormdb.DB, error) {
	db, err := repository.connection(ctx)
	if err != nil {
		return nil, err
	}
	return db.WithContext(ctx).Table("content_translations").Select("content_items.id AS content_id, content_translations.id AS translation_id, content_items.account_id, content_translations.locale_id, content_items.kind, content_items.content_key, content_items.status AS content_status, content_items.version AS content_version, content_translations.status AS translation_status, content_translations.version AS translation_version, content_translations.slug, content_translations.path, content_translations.title, content_translations.excerpt, content_translations.blocks, content_translations.seo_title, content_translations.seo_description, content_translations.geo_region, content_translations.geo_locality, content_translations.aeo_question, content_translations.aeo_answer, content_translations.published_at, content_translations.updated_at").Joins(contentJoin), nil
}

func (repository *ContentRepository) encodeIDs(accountID, contentID, localeID string) (any, any, any, error) {
	account, err := databaseid.Encode(repository.db.Dialector.Name(), accountID)
	if err != nil {
		return nil, nil, nil, err
	}
	content, err := databaseid.Encode(repository.db.Dialector.Name(), contentID)
	if err != nil {
		return nil, nil, nil, err
	}
	locale, err := databaseid.Encode(repository.db.Dialector.Name(), localeID)
	if err != nil {
		return nil, nil, nil, err
	}
	return account, content, locale, nil
}

func (repository *ContentRepository) encodeAccountLocaleIDs(accountID, localeID string) (any, any, error) {
	account, err := databaseid.Encode(repository.db.Dialector.Name(), accountID)
	if err != nil {
		return nil, nil, err
	}
	locale, err := databaseid.Encode(repository.db.Dialector.Name(), localeID)
	if err != nil {
		return nil, nil, err
	}
	return account, locale, nil
}

func listPage(records []contentListRecord, limit int) (ports.ContentListPage, error) {
	page := ports.ContentListPage{Items: make([]ports.ContentListItem, 0, limit)}
	if len(records) > limit {
		records = records[:limit]
		last := records[len(records)-1]
		page.Next = &ports.ContentCursor{UpdatedAt: last.UpdatedAt.UTC(), ID: last.TranslationID.String()}
	}
	for _, record := range records {
		item, mapErr := mapContentListRecord(record)
		if mapErr != nil {
			return ports.ContentListPage{}, mapErr
		}
		page.Items = append(page.Items, item)
	}
	return page, nil
}
