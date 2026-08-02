package gorm

import (
	"context"
	"fmt"
	"time"

	discoverabilitydomain "github.com/korwlik23/go-api-starter/internal/modules/discoverability/domain"
	"github.com/korwlik23/go-api-starter/internal/modules/discoverability/ports"
	"github.com/korwlik23/go-api-starter/internal/platform/database"
	"github.com/korwlik23/go-api-starter/internal/platform/databaseid"
	gormdb "gorm.io/gorm"
)

type RedirectRepository struct {
	db          *gormdb.DB
	connections database.ConnectionProvider
}

type redirectRecord struct {
	ID              databaseid.ID `gorm:"column:id"`
	AccountID       databaseid.ID `gorm:"column:account_id"`
	LocaleID        databaseid.ID `gorm:"column:locale_id"`
	SourcePath      string        `gorm:"column:source_path"`
	DestinationPath string        `gorm:"column:destination_path"`
	StatusCode      int           `gorm:"column:status_code"`
	Enabled         bool          `gorm:"column:enabled"`
	Version         int64         `gorm:"column:version"`
	DeletedAt       *time.Time    `gorm:"column:deleted_at"`
	CreatedAt       time.Time     `gorm:"column:created_at"`
	UpdatedAt       time.Time     `gorm:"column:updated_at"`
}
type seoRecord struct {
	ID            databaseid.ID `gorm:"column:id"`
	AccountID     databaseid.ID `gorm:"column:account_id"`
	LocaleID      databaseid.ID `gorm:"column:locale_id"`
	Title         string        `gorm:"column:title"`
	Description   *string       `gorm:"column:description"`
	CanonicalBase string        `gorm:"column:canonical_base"`
	Robots        string        `gorm:"column:robots"`
	Version       int64         `gorm:"column:version"`
	CreatedAt     time.Time     `gorm:"column:created_at"`
	UpdatedAt     time.Time     `gorm:"column:updated_at"`
}

func NewRedirectRepository(db *gormdb.DB) (*RedirectRepository, error) {
	if db == nil {
		return nil, ports.ErrInvalidRedirectRepository
	}
	manager, err := database.NewTransactionManager(db)
	if err != nil {
		return nil, ports.ErrInvalidRedirectRepository
	}
	return &RedirectRepository{db: db, connections: manager}, nil
}
func NewRedirectRepositoryWithDependencies(db *gormdb.DB, connections database.ConnectionProvider) *RedirectRepository {
	if db == nil || connections == nil {
		return nil
	}
	return &RedirectRepository{db: db, connections: connections}
}
func (repository *RedirectRepository) connection(ctx context.Context) (*gormdb.DB, error) {
	if repository == nil || repository.db == nil || repository.connections == nil || ctx == nil {
		return nil, ports.ErrInvalidRedirectRepository
	}
	return repository.connections.Connection(ctx)
}

func (repository *RedirectRepository) List(ctx context.Context, query ports.RedirectListQuery) (ports.RedirectListPage, error) {
	if repository == nil || repository.db == nil || ctx == nil {
		return ports.RedirectListPage{}, ports.ErrInvalidRedirectRepository
	}
	if err := ports.ValidateRedirectListQuery(query); err != nil {
		return ports.RedirectListPage{}, err
	}
	db, err := repository.connection(ctx)
	if err != nil {
		return ports.RedirectListPage{}, err
	}
	accountID, err := databaseid.Encode(repository.db.Dialector.Name(), query.AccountID)
	if err != nil {
		return ports.RedirectListPage{}, ports.ErrInvalidRedirectQuery
	}
	db = db.WithContext(ctx).Table("discoverability_redirects").Select("id, account_id, locale_id, source_path, destination_path, status_code, enabled, version, deleted_at, created_at, updated_at").Where("account_id = ? AND deleted_at IS NULL", accountID)
	if query.LocaleID != "" {
		localeID, encodeErr := databaseid.Encode(repository.db.Dialector.Name(), query.LocaleID)
		if encodeErr != nil {
			return ports.RedirectListPage{}, ports.ErrInvalidRedirectQuery
		}
		db = db.Where("locale_id = ?", localeID)
	}
	if query.Enabled != nil {
		db = db.Where("enabled = ?", *query.Enabled)
	}
	if query.After != nil {
		cursorID, encodeErr := databaseid.Encode(repository.db.Dialector.Name(), query.After.ID)
		if encodeErr != nil {
			return ports.RedirectListPage{}, ports.ErrInvalidRedirectQuery
		}
		db = db.Where("(updated_at < ? OR (updated_at = ? AND id < ?))", query.After.UpdatedAt, query.After.UpdatedAt, cursorID)
	}
	var records []redirectRecord
	if err := db.Order("updated_at DESC, id DESC").Limit(query.Limit + 1).Scan(&records).Error; err != nil {
		return ports.RedirectListPage{}, fmt.Errorf("list discoverability redirects: %w", err)
	}
	page := ports.RedirectListPage{Items: make([]discoverabilitydomain.Redirect, 0, query.Limit)}
	if len(records) > query.Limit {
		records = records[:query.Limit]
		last := records[len(records)-1]
		page.Next = &ports.RedirectCursor{UpdatedAt: last.UpdatedAt.UTC(), ID: last.ID.String()}
	}
	for _, record := range records {
		redirect, mapErr := redirectFromRecord(record)
		if mapErr != nil {
			return ports.RedirectListPage{}, mapErr
		}
		page.Items = append(page.Items, redirect)
	}
	return page, nil
}

func (repository *RedirectRepository) Get(ctx context.Context, accountID, redirectID string) (discoverabilitydomain.Redirect, error) {
	if repository == nil || repository.db == nil || ctx == nil || ports.ValidateRedirectIdentity(accountID, redirectID) != nil {
		return discoverabilitydomain.Redirect{}, ports.ErrRedirectNotFound
	}
	db, err := repository.connection(ctx)
	if err != nil {
		return discoverabilitydomain.Redirect{}, err
	}
	encodedAccount, err := databaseid.Encode(repository.db.Dialector.Name(), accountID)
	if err != nil {
		return discoverabilitydomain.Redirect{}, ports.ErrRedirectNotFound
	}
	encodedID, err := databaseid.Encode(repository.db.Dialector.Name(), redirectID)
	if err != nil {
		return discoverabilitydomain.Redirect{}, ports.ErrRedirectNotFound
	}
	var record redirectRecord
	result := db.WithContext(ctx).Table("discoverability_redirects").Select("id, account_id, locale_id, source_path, destination_path, status_code, enabled, version, deleted_at, created_at, updated_at").Where("account_id = ? AND id = ? AND deleted_at IS NULL", encodedAccount, encodedID).Limit(1).Scan(&record)
	if result.Error != nil {
		return discoverabilitydomain.Redirect{}, fmt.Errorf("get discoverability redirect: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return discoverabilitydomain.Redirect{}, ports.ErrRedirectNotFound
	}
	return redirectFromRecord(record)
}

func (repository *RedirectRepository) Create(ctx context.Context, command ports.CreateRedirectCommand) (discoverabilitydomain.Redirect, error) {
	db, err := repository.connection(ctx)
	if err != nil {
		return discoverabilitydomain.Redirect{}, err
	}
	accountID, err := databaseid.Encode(repository.db.Dialector.Name(), command.Redirect.AccountID)
	if err != nil {
		return discoverabilitydomain.Redirect{}, ports.ErrInvalidRedirectRepository
	}
	id, err := databaseid.Encode(repository.db.Dialector.Name(), command.Redirect.ID)
	if err != nil {
		return discoverabilitydomain.Redirect{}, ports.ErrInvalidRedirectRepository
	}
	localeID, err := databaseid.Encode(repository.db.Dialector.Name(), command.Redirect.LocaleID)
	if err != nil {
		return discoverabilitydomain.Redirect{}, ports.ErrInvalidRedirectRepository
	}
	if err := db.WithContext(ctx).Table("discoverability_redirects").Create(map[string]any{"id": id, "account_id": accountID, "locale_id": localeID, "source_path": command.Redirect.SourcePath, "destination_path": command.Redirect.DestinationPath, "status_code": command.Redirect.StatusCode, "enabled": command.Redirect.Enabled, "version": command.Redirect.Version, "created_at": command.Redirect.CreatedAt, "updated_at": command.Redirect.UpdatedAt}).Error; err != nil {
		return discoverabilitydomain.Redirect{}, fmt.Errorf("create discoverability redirect: %w", err)
	}
	return repository.Get(ctx, command.Redirect.AccountID, command.Redirect.ID)
}

func (repository *RedirectRepository) Update(ctx context.Context, command ports.UpdateRedirectCommand) (discoverabilitydomain.Redirect, error) {
	db, err := repository.connection(ctx)
	if err != nil {
		return discoverabilitydomain.Redirect{}, err
	}
	accountID, err := databaseid.Encode(repository.db.Dialector.Name(), command.AccountID)
	if err != nil {
		return discoverabilitydomain.Redirect{}, ports.ErrRedirectNotFound
	}
	id, err := databaseid.Encode(repository.db.Dialector.Name(), command.RedirectID)
	if err != nil {
		return discoverabilitydomain.Redirect{}, ports.ErrRedirectNotFound
	}
	localeID, err := databaseid.Encode(repository.db.Dialector.Name(), command.Redirect.LocaleID)
	if err != nil {
		return discoverabilitydomain.Redirect{}, ports.ErrInvalidRedirectRepository
	}
	result := db.WithContext(ctx).Table("discoverability_redirects").Where("account_id = ? AND id = ? AND version = ? AND deleted_at IS NULL", accountID, id, command.ExpectedVersion).Updates(map[string]any{"locale_id": localeID, "source_path": command.Redirect.SourcePath, "destination_path": command.Redirect.DestinationPath, "status_code": command.Redirect.StatusCode, "enabled": command.Redirect.Enabled, "version": command.Redirect.Version, "updated_at": command.Redirect.UpdatedAt})
	if result.Error != nil {
		return discoverabilitydomain.Redirect{}, fmt.Errorf("update discoverability redirect: %w", result.Error)
	}
	if result.RowsAffected != 1 {
		return discoverabilitydomain.Redirect{}, ports.ErrRedirectConflict
	}
	return repository.Get(ctx, command.AccountID, command.RedirectID)
}

func (repository *RedirectRepository) Delete(ctx context.Context, command ports.DeleteRedirectCommand) error {
	db, err := repository.connection(ctx)
	if err != nil {
		return err
	}
	accountID, err := databaseid.Encode(repository.db.Dialector.Name(), command.AccountID)
	if err != nil {
		return ports.ErrRedirectNotFound
	}
	id, err := databaseid.Encode(repository.db.Dialector.Name(), command.RedirectID)
	if err != nil {
		return ports.ErrRedirectNotFound
	}
	result := db.WithContext(ctx).Table("discoverability_redirects").Where("account_id = ? AND id = ? AND version = ? AND deleted_at IS NULL", accountID, id, command.ExpectedVersion).Updates(map[string]any{"deleted_at": command.DeletedAt, "version": command.ExpectedVersion + 1, "updated_at": command.DeletedAt})
	if result.Error != nil {
		return fmt.Errorf("delete discoverability redirect: %w", result.Error)
	}
	if result.RowsAffected != 1 {
		return ports.ErrRedirectConflict
	}
	return nil
}

func (repository *RedirectRepository) GetSEODefaults(ctx context.Context, accountID, localeID string) (discoverabilitydomain.SEODefaults, error) {
	if repository == nil || repository.db == nil || ctx == nil || !validIDs(accountID, localeID) {
		return discoverabilitydomain.SEODefaults{}, ports.ErrSEODefaultsNotFound
	}
	db, err := repository.connection(ctx)
	if err != nil {
		return discoverabilitydomain.SEODefaults{}, err
	}
	encodedAccount, err := databaseid.Encode(repository.db.Dialector.Name(), accountID)
	if err != nil {
		return discoverabilitydomain.SEODefaults{}, ports.ErrSEODefaultsNotFound
	}
	encodedLocale, err := databaseid.Encode(repository.db.Dialector.Name(), localeID)
	if err != nil {
		return discoverabilitydomain.SEODefaults{}, ports.ErrSEODefaultsNotFound
	}
	var record seoRecord
	result := db.WithContext(ctx).Table("discoverability_seo_defaults").Select("id, account_id, locale_id, title, description, canonical_base, robots, version, created_at, updated_at").Where("account_id = ? AND locale_id = ?", encodedAccount, encodedLocale).Limit(1).Scan(&record)
	if result.Error != nil {
		return discoverabilitydomain.SEODefaults{}, fmt.Errorf("get SEO defaults: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return discoverabilitydomain.SEODefaults{}, ports.ErrSEODefaultsNotFound
	}
	return seoFromRecord(record)
}

func (repository *RedirectRepository) UpsertSEODefaults(ctx context.Context, command ports.UpsertSEODefaultsCommand) (discoverabilitydomain.SEODefaults, error) {
	db, err := repository.connection(ctx)
	if err != nil {
		return discoverabilitydomain.SEODefaults{}, err
	}
	accountID, err := databaseid.Encode(repository.db.Dialector.Name(), command.Defaults.AccountID)
	if err != nil {
		return discoverabilitydomain.SEODefaults{}, ports.ErrInvalidRedirectRepository
	}
	localeID, err := databaseid.Encode(repository.db.Dialector.Name(), command.Defaults.LocaleID)
	if err != nil {
		return discoverabilitydomain.SEODefaults{}, ports.ErrInvalidRedirectRepository
	}
	if command.ExpectedVersion == 0 {
		id, encodeErr := databaseid.Encode(repository.db.Dialector.Name(), command.Defaults.ID)
		if encodeErr != nil {
			return discoverabilitydomain.SEODefaults{}, ports.ErrInvalidRedirectRepository
		}
		if err := db.WithContext(ctx).Table("discoverability_seo_defaults").Create(map[string]any{"id": id, "account_id": accountID, "locale_id": localeID, "title": command.Defaults.Title, "description": nullableString(command.Defaults.Description), "canonical_base": command.Defaults.CanonicalBase, "robots": command.Defaults.Robots, "version": command.Defaults.Version, "created_at": command.Defaults.CreatedAt, "updated_at": command.Defaults.UpdatedAt}).Error; err != nil {
			return discoverabilitydomain.SEODefaults{}, ports.ErrSEODefaultsConflict
		}
		return repository.GetSEODefaults(ctx, command.Defaults.AccountID, command.Defaults.LocaleID)
	}
	result := db.WithContext(ctx).Table("discoverability_seo_defaults").Where("account_id = ? AND locale_id = ? AND version = ?", accountID, localeID, command.ExpectedVersion).Updates(map[string]any{"title": command.Defaults.Title, "description": nullableString(command.Defaults.Description), "canonical_base": command.Defaults.CanonicalBase, "robots": command.Defaults.Robots, "version": command.Defaults.Version, "updated_at": command.Defaults.UpdatedAt})
	if result.Error != nil {
		return discoverabilitydomain.SEODefaults{}, fmt.Errorf("update SEO defaults: %w", result.Error)
	}
	if result.RowsAffected != 1 {
		return discoverabilitydomain.SEODefaults{}, ports.ErrSEODefaultsConflict
	}
	return repository.GetSEODefaults(ctx, command.Defaults.AccountID, command.Defaults.LocaleID)
}

func redirectFromRecord(record redirectRecord) (discoverabilitydomain.Redirect, error) {
	return discoverabilitydomain.NewRedirect(discoverabilitydomain.RedirectInput{ID: record.ID.String(), AccountID: record.AccountID.String(), LocaleID: record.LocaleID.String(), SourcePath: record.SourcePath, DestinationPath: record.DestinationPath, StatusCode: discoverabilitydomain.StatusCode(record.StatusCode), Enabled: record.Enabled, Version: record.Version, DeletedAt: record.DeletedAt, CreatedAt: record.CreatedAt, UpdatedAt: record.UpdatedAt})
}
func seoFromRecord(record seoRecord) (discoverabilitydomain.SEODefaults, error) {
	description := ""
	if record.Description != nil {
		description = *record.Description
	}
	return discoverabilitydomain.NewSEODefaults(discoverabilitydomain.SEODefaultsInput{ID: record.ID.String(), AccountID: record.AccountID.String(), LocaleID: record.LocaleID.String(), Title: record.Title, Description: description, CanonicalBase: record.CanonicalBase, Robots: record.Robots, Version: record.Version, CreatedAt: record.CreatedAt, UpdatedAt: record.UpdatedAt})
}
func validIDs(accountID, localeID string) bool {
	return ports.ValidateRedirectListQuery(ports.RedirectListQuery{AccountID: accountID, LocaleID: localeID, Limit: 1}) == nil
}
func nullableString(value string) any {
	if value == "" {
		return nil
	}
	return value
}

var _ ports.RedirectRepository = (*RedirectRepository)(nil)
var _ ports.SEODefaultsRepository = (*RedirectRepository)(nil)
