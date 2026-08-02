package gorm

import (
	"encoding/json"
	"time"

	publishingdomain "github.com/korwlik23/go-api-starter/internal/modules/publishing/domain"
	"github.com/korwlik23/go-api-starter/internal/modules/publishing/ports"
	"github.com/korwlik23/go-api-starter/internal/platform/databaseid"
)

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
	PublishedAt        *time.Time    `gorm:"column:published_at"`
	UpdatedAt          time.Time     `gorm:"column:updated_at"`
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
		ContentID: record.ContentID.String(), AccountID: record.AccountID.String(),
		Kind: publishingdomain.Kind(record.Kind), ContentKey: record.ContentKey,
		ContentStatus: publishingdomain.Status(record.ContentStatus), ContentVersion: record.ContentVersion,
		TranslationID: record.TranslationID.String(), LocaleID: record.LocaleID.String(),
		Slug: record.Slug, Path: record.Path, Title: record.Title,
		TranslationStatus:  publishingdomain.Status(record.TranslationStatus),
		TranslationVersion: record.TranslationVersion, UpdatedAt: record.UpdatedAt.UTC(),
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
		ContentID: record.ContentID.String(), TranslationID: record.TranslationID.String(),
		AccountID: record.AccountID.String(), LocaleID: record.LocaleID.String(),
		Kind: publishingdomain.Kind(record.Kind), ContentKey: record.ContentKey,
		ContentStatus: publishingdomain.Status(record.ContentStatus), ContentVersion: record.ContentVersion,
		TranslationStatus: publishingdomain.Status(record.TranslationStatus), TranslationVersion: record.TranslationVersion,
		Slug: record.Slug, Path: record.Path, Title: record.Title, Blocks: blocks,
		UpdatedAt: record.UpdatedAt.UTC(),
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
	if record.PublishedAt != nil {
		publishedAt := record.PublishedAt.UTC()
		detail.PublishedAt = &publishedAt
	}
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
