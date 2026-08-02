package gorm

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	discoverabilitydomain "github.com/korwlik23/go-api-starter/internal/modules/discoverability/domain"
	"github.com/korwlik23/go-api-starter/internal/modules/discoverability/ports"
	"github.com/korwlik23/go-api-starter/internal/platform/database"
	"github.com/korwlik23/go-api-starter/internal/platform/databaseid"
	gormdb "gorm.io/gorm"
)

type ContentAuditRepository struct {
	db          *gormdb.DB
	connections database.ConnectionProvider
}

type contentAuditRecord struct {
	ID                 databaseid.ID `gorm:"column:id"`
	AccountID          databaseid.ID `gorm:"column:account_id"`
	ContentID          databaseid.ID `gorm:"column:content_id"`
	TranslationID      databaseid.ID `gorm:"column:translation_id"`
	LocaleID           databaseid.ID `gorm:"column:locale_id"`
	ContentVersion     int64         `gorm:"column:content_version"`
	TranslationVersion int64         `gorm:"column:translation_version"`
	Status             string        `gorm:"column:status"`
	Checks             []byte        `gorm:"column:checks"`
	CreatedAt          time.Time     `gorm:"column:created_at"`
	UpdatedAt          time.Time     `gorm:"column:updated_at"`
}

func NewContentAuditRepository(db *gormdb.DB) (*ContentAuditRepository, error) {
	if db == nil {
		return nil, ports.ErrInvalidContentAuditRepository
	}
	manager, err := database.NewTransactionManager(db)
	if err != nil {
		return nil, ports.ErrInvalidContentAuditRepository
	}
	return &ContentAuditRepository{db: db, connections: manager}, nil
}

func NewContentAuditRepositoryWithDependencies(db *gormdb.DB, connections database.ConnectionProvider) *ContentAuditRepository {
	if db == nil || connections == nil {
		return nil
	}
	return &ContentAuditRepository{db: db, connections: connections}
}

func (repository *ContentAuditRepository) connection(ctx context.Context) (*gormdb.DB, error) {
	if repository == nil || repository.db == nil || repository.connections == nil || ctx == nil {
		return nil, ports.ErrInvalidContentAuditRepository
	}
	return repository.connections.Connection(ctx)
}

func (repository *ContentAuditRepository) Save(ctx context.Context, command ports.SaveContentAuditCommand) (discoverabilitydomain.ContentAudit, error) {
	if repository == nil || repository.db == nil || ctx == nil {
		return discoverabilitydomain.ContentAudit{}, ports.ErrInvalidContentAuditRepository
	}
	if _, err := discoverabilitydomain.NewContentAudit(discoverabilitydomain.ContentAuditInput{
		ID: command.Audit.ID, AccountID: command.Audit.AccountID, ContentID: command.Audit.ContentID, TranslationID: command.Audit.TranslationID, LocaleID: command.Audit.LocaleID,
		ContentVersion: command.Audit.ContentVersion, TranslationVersion: command.Audit.TranslationVersion, Status: command.Audit.Status, Checks: command.Audit.Checks, CreatedAt: command.Audit.CreatedAt, UpdatedAt: command.Audit.UpdatedAt,
	}); err != nil {
		return discoverabilitydomain.ContentAudit{}, err
	}
	db, err := repository.connection(ctx)
	if err != nil {
		return discoverabilitydomain.ContentAudit{}, err
	}
	encodedIDs, err := repository.encodeIDs(command.Audit.AccountID, command.Audit.ContentID, command.Audit.TranslationID, command.Audit.LocaleID)
	if err != nil {
		return discoverabilitydomain.ContentAudit{}, ports.ErrInvalidContentAuditRepository
	}
	accountID, contentID, translationID, localeID := encodedIDs[0], encodedIDs[1], encodedIDs[2], encodedIDs[3]
	var existing contentAuditRecord
	lookup := db.WithContext(ctx).Table("discoverability_content_audits").Select("id, account_id, content_id, translation_id, locale_id, content_version, translation_version, status, checks, created_at, updated_at").Where("account_id = ? AND translation_id = ? AND content_version = ? AND translation_version = ?", accountID, translationID, command.Audit.ContentVersion, command.Audit.TranslationVersion).Limit(1).Scan(&existing)
	if lookup.Error != nil {
		return discoverabilitydomain.ContentAudit{}, fmt.Errorf("find existing content audit: %w", lookup.Error)
	}
	if lookup.RowsAffected == 1 {
		return contentAuditFromRecord(existing)
	}
	checks, err := json.Marshal(command.Audit.Checks)
	if err != nil {
		return discoverabilitydomain.ContentAudit{}, ports.ErrInvalidContentAuditRepository
	}
	result := db.WithContext(ctx).Table("discoverability_content_audits").Create(map[string]any{
		"id": idValue(repository.db.Dialector.Name(), command.Audit.ID), "account_id": accountID, "content_id": contentID, "translation_id": translationID, "locale_id": localeID,
		"content_version": command.Audit.ContentVersion, "translation_version": command.Audit.TranslationVersion, "status": command.Audit.Status, "checks": checks,
		"created_at": command.Audit.CreatedAt, "updated_at": command.Audit.UpdatedAt,
	})
	if result.Error != nil {
		return discoverabilitydomain.ContentAudit{}, ports.ErrContentAuditConflict
	}
	return repository.Get(ctx, command.Audit.AccountID, command.Audit.ID)
}

func (repository *ContentAuditRepository) Get(ctx context.Context, accountID, auditID string) (discoverabilitydomain.ContentAudit, error) {
	if repository == nil || repository.db == nil || ctx == nil || ports.ValidateContentAuditIdentity(accountID, auditID) != nil {
		return discoverabilitydomain.ContentAudit{}, ports.ErrContentAuditNotFound
	}
	db, err := repository.connection(ctx)
	if err != nil {
		return discoverabilitydomain.ContentAudit{}, err
	}
	encodedIDs, err := repository.encodeIDs(accountID, auditID)
	if err != nil {
		return discoverabilitydomain.ContentAudit{}, ports.ErrContentAuditNotFound
	}
	encodedAccount, encodedID := encodedIDs[0], encodedIDs[1]
	var record contentAuditRecord
	result := db.WithContext(ctx).Table("discoverability_content_audits").Select("id, account_id, content_id, translation_id, locale_id, content_version, translation_version, status, checks, created_at, updated_at").Where("account_id = ? AND id = ?", encodedAccount, encodedID).Limit(1).Scan(&record)
	if result.Error != nil {
		return discoverabilitydomain.ContentAudit{}, fmt.Errorf("get content audit: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return discoverabilitydomain.ContentAudit{}, ports.ErrContentAuditNotFound
	}
	return contentAuditFromRecord(record)
}

func (repository *ContentAuditRepository) List(ctx context.Context, query ports.ContentAuditListQuery) (ports.ContentAuditListPage, error) {
	if repository == nil || repository.db == nil || ctx == nil {
		return ports.ContentAuditListPage{}, ports.ErrInvalidContentAuditRepository
	}
	if err := ports.ValidateContentAuditListQuery(query); err != nil {
		return ports.ContentAuditListPage{}, err
	}
	db, err := repository.connection(ctx)
	if err != nil {
		return ports.ContentAuditListPage{}, err
	}
	accountID, err := databaseid.Encode(repository.db.Dialector.Name(), query.AccountID)
	if err != nil {
		return ports.ContentAuditListPage{}, ports.ErrInvalidContentAuditQuery
	}
	db = db.WithContext(ctx).Table("discoverability_content_audits").Select("id, account_id, content_id, translation_id, locale_id, content_version, translation_version, status, checks, created_at, updated_at").Where("account_id = ?", accountID)
	filters := []struct {
		column string
		value  string
	}{
		{column: "content_id", value: query.ContentID},
		{column: "translation_id", value: query.TranslationID},
		{column: "locale_id", value: query.LocaleID},
	}
	for _, filter := range filters {
		column, value := filter.column, filter.value
		if value == "" {
			continue
		}
		encoded, encodeErr := databaseid.Encode(repository.db.Dialector.Name(), value)
		if encodeErr != nil {
			return ports.ContentAuditListPage{}, ports.ErrInvalidContentAuditQuery
		}
		db = db.Where(column+" = ?", encoded)
	}
	if query.Status != nil {
		db = db.Where("status = ?", *query.Status)
	}
	if query.After != nil {
		cursorID, encodeErr := databaseid.Encode(repository.db.Dialector.Name(), query.After.ID)
		if encodeErr != nil {
			return ports.ContentAuditListPage{}, ports.ErrInvalidContentAuditQuery
		}
		db = db.Where("(updated_at < ? OR (updated_at = ? AND id < ?))", query.After.UpdatedAt, query.After.UpdatedAt, cursorID)
	}
	var records []contentAuditRecord
	if err := db.Order("updated_at DESC, id DESC").Limit(query.Limit + 1).Scan(&records).Error; err != nil {
		return ports.ContentAuditListPage{}, fmt.Errorf("list content audits: %w", err)
	}
	page := ports.ContentAuditListPage{Items: make([]discoverabilitydomain.ContentAudit, 0, query.Limit)}
	if len(records) > query.Limit {
		records = records[:query.Limit]
		last := records[len(records)-1]
		page.Next = &ports.ContentAuditCursor{UpdatedAt: last.UpdatedAt.UTC(), ID: last.ID.String()}
	}
	for _, record := range records {
		audit, mapErr := contentAuditFromRecord(record)
		if mapErr != nil {
			return ports.ContentAuditListPage{}, mapErr
		}
		page.Items = append(page.Items, audit)
	}
	return page, nil
}

func (repository *ContentAuditRepository) encodeIDs(values ...string) ([]any, error) {
	encoded := make([]any, len(values))
	for index, value := range values {
		item, err := databaseid.Encode(repository.db.Dialector.Name(), value)
		if err != nil {
			return nil, err
		}
		encoded[index] = item
	}
	return encoded, nil
}

func idValue(dialect, value string) any {
	encoded, err := databaseid.Encode(dialect, value)
	if err != nil {
		return value
	}
	return encoded
}

func contentAuditFromRecord(record contentAuditRecord) (discoverabilitydomain.ContentAudit, error) {
	var checks []discoverabilitydomain.AuditCheck
	if len(record.Checks) == 0 || json.Unmarshal(record.Checks, &checks) != nil {
		return discoverabilitydomain.ContentAudit{}, ports.ErrInvalidContentAuditRepository
	}
	return discoverabilitydomain.NewContentAudit(discoverabilitydomain.ContentAuditInput{
		ID: record.ID.String(), AccountID: record.AccountID.String(), ContentID: record.ContentID.String(), TranslationID: record.TranslationID.String(), LocaleID: record.LocaleID.String(),
		ContentVersion: record.ContentVersion, TranslationVersion: record.TranslationVersion, Status: discoverabilitydomain.AuditStatus(record.Status), Checks: checks, CreatedAt: record.CreatedAt, UpdatedAt: record.UpdatedAt,
	})
}

var _ ports.ContentAuditRepository = (*ContentAuditRepository)(nil)
