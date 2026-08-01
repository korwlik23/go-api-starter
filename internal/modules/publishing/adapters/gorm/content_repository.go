package gorm

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	publishingdomain "github.com/korwlik23/go-api-starter/internal/modules/publishing/domain"
	"github.com/korwlik23/go-api-starter/internal/modules/publishing/ports"
	"github.com/korwlik23/go-api-starter/internal/platform/databaseid"
	gormdb "gorm.io/gorm"
)

var ErrInvalidContentRepository = errors.New("invalid content repository")

type ContentRepository struct {
	db *gormdb.DB
}

type contentListRecord struct {
	ContentID          databaseid.ID `gorm:"column:content_id"`
	AccountID          databaseid.ID `gorm:"column:account_id"`
	Kind               string        `gorm:"column:kind"`
	ContentKey         string        `gorm:"column:content_key"`
	ContentStatus      string        `gorm:"column:content_status"`
	ContentVersion     int64         `gorm:"column:content_version"`
	TranslationID      databaseid.ID `gorm:"column:translation_id"`
	LocaleID           databaseid.ID `gorm:"column:locale_id"`
	Slug               string        `gorm:"column:slug"`
	Path               string        `gorm:"column:path"`
	Title              string        `gorm:"column:title"`
	TranslationStatus  string        `gorm:"column:translation_status"`
	TranslationVersion int64         `gorm:"column:translation_version"`
	UpdatedAt          time.Time     `gorm:"column:updated_at"`
}

type contentDetailRecord struct {
	ContentID          databaseid.ID `gorm:"column:content_id"`
	TranslationID      databaseid.ID `gorm:"column:translation_id"`
	AccountID          databaseid.ID `gorm:"column:account_id"`
	LocaleID           databaseid.ID `gorm:"column:locale_id"`
	Kind               string        `gorm:"column:kind"`
	ContentKey         string        `gorm:"column:content_key"`
	ContentStatus      string        `gorm:"column:content_status"`
	ContentVersion     int64         `gorm:"column:content_version"`
	TranslationStatus  string        `gorm:"column:translation_status"`
	TranslationVersion int64         `gorm:"column:translation_version"`
	Slug               string        `gorm:"column:slug"`
	Path               string        `gorm:"column:path"`
	Title              string        `gorm:"column:title"`
	Excerpt            *string       `gorm:"column:excerpt"`
	Blocks             []byte        `gorm:"column:blocks"`
	SEOTitle           *string       `gorm:"column:seo_title"`
	SEODescription     *string       `gorm:"column:seo_description"`
	GEORegion          *string       `gorm:"column:geo_region"`
	GEOLocality        *string       `gorm:"column:geo_locality"`
	AEOQuestion        *string       `gorm:"column:aeo_question"`
	AEOAnswer          *string       `gorm:"column:aeo_answer"`
	UpdatedAt          time.Time     `gorm:"column:updated_at"`
}

func (contentListRecord) tableName() string {
	return "content_translations"
}

func NewContentRepository(db *gormdb.DB) (*ContentRepository, error) {
	if db == nil {
		return nil, ErrInvalidContentRepository
	}
	return &ContentRepository{db: db}, nil
}

func (repository *ContentRepository) List(
	ctx context.Context,
	query ports.ContentListQuery,
) (ports.ContentListPage, error) {
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
	db := repository.db.WithContext(ctx).
		Table("content_translations").
		Select(
			"content_items.id AS content_id, content_items.account_id, "+
				"content_items.kind, content_items.content_key, "+
				"content_items.status AS content_status, content_items.version AS content_version, "+
				"content_translations.id AS translation_id, content_translations.locale_id, "+
				"content_translations.slug, content_translations.path, content_translations.title, "+
				"content_translations.status AS translation_status, "+
				"content_translations.version AS translation_version, "+
				"content_translations.updated_at").
		Joins(
			"JOIN content_items ON content_items.id = content_translations.content_id "+
				"AND content_items.account_id = content_translations.account_id").
		Where(
			"content_items.account_id = ? AND content_translations.locale_id = ? "+
				"AND content_items.deleted_at IS NULL AND content_translations.deleted_at IS NULL",
			accountID,
			localeID,
		)
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
		cursorID, encodeErr := databaseid.Encode(
			repository.db.Dialector.Name(), query.After.ID,
		)
		if encodeErr != nil {
			return ports.ContentListPage{}, ports.ErrInvalidContentQuery
		}
		db = db.Where(
			"(content_translations.updated_at < ? OR "+
				"(content_translations.updated_at = ? AND content_translations.id < ?))",
			query.After.UpdatedAt,
			query.After.UpdatedAt,
			cursorID,
		)
	}
	var records []contentListRecord
	if err := db.
		Order("content_translations.updated_at DESC, content_translations.id DESC").
		Limit(query.Limit + 1).
		Scan(&records).Error; err != nil {
		return ports.ContentListPage{}, fmt.Errorf("list content: %w", err)
	}
	page := ports.ContentListPage{Items: make([]ports.ContentListItem, 0, query.Limit)}
	if len(records) > query.Limit {
		records = records[:query.Limit]
		last := records[len(records)-1]
		page.Next = &ports.ContentCursor{
			UpdatedAt: last.UpdatedAt.UTC(),
			ID:        last.TranslationID.String(),
		}
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

func (repository *ContentRepository) Get(
	ctx context.Context,
	accountID string,
	contentID string,
	localeID string,
) (ports.ContentDetail, error) {
	if repository == nil || repository.db == nil || ctx == nil {
		return ports.ContentDetail{}, ErrInvalidContentRepository
	}
	encodedAccountID, encodedContentID, encodedLocaleID, err :=
		repository.encodeIDs(accountID, contentID, localeID)
	if err != nil {
		return ports.ContentDetail{}, ports.ErrContentNotFound
	}
	var record contentDetailRecord
	db := repository.detailQuery(ctx).
		Where(
			"content_items.account_id = ? AND content_translations.content_id = ? "+
				"AND content_translations.locale_id = ? "+
				"AND content_items.deleted_at IS NULL AND content_translations.deleted_at IS NULL",
			encodedAccountID,
			encodedContentID,
			encodedLocaleID,
		).
		Limit(1)
	result := db.Scan(&record)
	if result.Error != nil {
		return ports.ContentDetail{}, fmt.Errorf("get content: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return ports.ContentDetail{}, ports.ErrContentNotFound
	}
	return mapContentDetailRecord(record)
}

func (repository *ContentRepository) FindPublic(
	ctx context.Context,
	accountID string,
	localeID string,
	path string,
) (ports.ContentDetail, error) {
	if repository == nil || repository.db == nil || ctx == nil {
		return ports.ContentDetail{}, ErrInvalidContentRepository
	}
	normalizedPath, err := publishingdomain.NormalizePath(path)
	if err != nil {
		return ports.ContentDetail{}, ports.ErrContentNotFound
	}
	encodedAccountID, encodedLocaleID, err := repository.encodeAccountLocaleIDs(
		accountID,
		localeID,
	)
	if err != nil {
		return ports.ContentDetail{}, ports.ErrContentNotFound
	}
	var record contentDetailRecord
	result := repository.detailQuery(ctx).
		Where(
			"content_items.account_id = ? AND content_translations.locale_id = ? "+
				"AND content_translations.path = ? AND content_items.status = ? "+
				"AND content_translations.status = ? "+
				"AND content_items.deleted_at IS NULL AND content_translations.deleted_at IS NULL",
			encodedAccountID,
			encodedLocaleID,
			normalizedPath,
			string(publishingdomain.StatusPublished),
			string(publishingdomain.StatusPublished),
		).
		Limit(1).
		Scan(&record)
	if result.Error != nil {
		return ports.ContentDetail{}, fmt.Errorf("find public content: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return ports.ContentDetail{}, ports.ErrContentNotFound
	}
	return mapContentDetailRecord(record)
}

func (repository *ContentRepository) detailQuery(ctx context.Context) *gormdb.DB {
	return repository.db.WithContext(ctx).
		Table("content_translations").
		Select(
			"content_items.id AS content_id, content_translations.id AS translation_id, " +
				"content_items.account_id, content_translations.locale_id, content_items.kind, " +
				"content_items.content_key, content_items.status AS content_status, " +
				"content_items.version AS content_version, " +
				"content_translations.status AS translation_status, " +
				"content_translations.version AS translation_version, " +
				"content_translations.slug, content_translations.path, " +
				"content_translations.title, content_translations.excerpt, " +
				"content_translations.blocks, content_translations.seo_title, " +
				"content_translations.seo_description, content_translations.geo_region, " +
				"content_translations.geo_locality, content_translations.aeo_question, " +
				"content_translations.aeo_answer, content_translations.updated_at").
		Joins(
			"JOIN content_items ON content_items.id = content_translations.content_id " +
				"AND content_items.account_id = content_translations.account_id")
}

func (repository *ContentRepository) encodeIDs(
	accountID string,
	contentID string,
	localeID string,
) (any, any, any, error) {
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

func (repository *ContentRepository) encodeAccountLocaleIDs(
	accountID string,
	localeID string,
) (any, any, error) {
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

func mapContentListRecord(record contentListRecord) (ports.ContentListItem, error) {
	if record.ContentID.String() == "" || record.AccountID.String() == "" ||
		record.TranslationID.String() == "" || record.LocaleID.String() == "" ||
		record.ContentVersion < 1 || record.TranslationVersion < 1 ||
		!validKind(record.Kind) || !validStatus(record.ContentStatus) ||
		!validStatus(record.TranslationStatus) || record.UpdatedAt.IsZero() {
		return ports.ContentListItem{}, ErrInvalidContentRepository
	}
	return ports.ContentListItem{
		ContentID:          record.ContentID.String(),
		AccountID:          record.AccountID.String(),
		Kind:               publishingdomain.Kind(record.Kind),
		ContentKey:         record.ContentKey,
		ContentStatus:      publishingdomain.Status(record.ContentStatus),
		ContentVersion:     record.ContentVersion,
		TranslationID:      record.TranslationID.String(),
		LocaleID:           record.LocaleID.String(),
		Slug:               record.Slug,
		Path:               record.Path,
		Title:              record.Title,
		TranslationStatus:  publishingdomain.Status(record.TranslationStatus),
		TranslationVersion: record.TranslationVersion,
		UpdatedAt:          record.UpdatedAt.UTC(),
	}, nil
}

func mapContentDetailRecord(record contentDetailRecord) (ports.ContentDetail, error) {
	if record.ContentID.String() == "" || record.TranslationID.String() == "" ||
		record.AccountID.String() == "" || record.LocaleID.String() == "" ||
		record.ContentVersion < 1 || record.TranslationVersion < 1 ||
		!validKind(record.Kind) || !validStatus(record.ContentStatus) ||
		!validStatus(record.TranslationStatus) || record.UpdatedAt.IsZero() {
		return ports.ContentDetail{}, ErrInvalidContentRepository
	}
	var blocks []publishingdomain.Block
	if len(record.Blocks) == 0 || json.Unmarshal(record.Blocks, &blocks) != nil {
		return ports.ContentDetail{}, ErrInvalidContentRepository
	}
	detail := ports.ContentDetail{
		ContentID:          record.ContentID.String(),
		TranslationID:      record.TranslationID.String(),
		AccountID:          record.AccountID.String(),
		LocaleID:           record.LocaleID.String(),
		Kind:               publishingdomain.Kind(record.Kind),
		ContentKey:         record.ContentKey,
		ContentStatus:      publishingdomain.Status(record.ContentStatus),
		ContentVersion:     record.ContentVersion,
		TranslationStatus:  publishingdomain.Status(record.TranslationStatus),
		TranslationVersion: record.TranslationVersion,
		Slug:               record.Slug,
		Path:               record.Path,
		Title:              record.Title,
		Blocks:             blocks,
		UpdatedAt:          record.UpdatedAt.UTC(),
	}
	if record.Excerpt != nil {
		detail.Excerpt = *record.Excerpt
	}
	detail.SEO.Title = stringPointer(record.SEOTitle)
	detail.SEO.Description = stringPointer(record.SEODescription)
	detail.GEO.Region = stringPointer(record.GEORegion)
	detail.GEO.Locality = stringPointer(record.GEOLocality)
	detail.AEO.Question = stringPointer(record.AEOQuestion)
	detail.AEO.Answer = stringPointer(record.AEOAnswer)
	return detail, nil
}

func stringPointer(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func validKind(value string) bool {
	return value == string(publishingdomain.KindPage) || value == string(publishingdomain.KindPost)
}

func validStatus(value string) bool {
	return publishingdomain.ValidateStatus(publishingdomain.Status(value)) == nil
}

var _ ports.ContentRepository = (*ContentRepository)(nil)
