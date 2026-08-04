package catalog

import (
	"context"
	"errors"
	"strings"

	authorizationdomain "github.com/korwlik23/go-api-starter/internal/modules/authorization/domain"
	"github.com/korwlik23/go-api-starter/internal/modules/localization/ports"
	"github.com/korwlik23/go-api-starter/internal/platform/database"
)

var ErrInvalidImportExport = errors.New("localization: invalid catalog import/export")

type ImportExport struct {
	authorizer      systemPermissionAuthorizer
	catalog         ports.CatalogRepository
	updates         ports.CatalogUpdateRepository
	transactions    database.Transactor
	auditor         CatalogAuditor
	readPermission  authorizationdomain.PermissionKey
	writePermission authorizationdomain.PermissionKey
}

type ExportResult struct {
	Locale   string            `json:"locale"`
	Category string            `json:"category"`
	Version  int64             `json:"version"`
	Entries  map[string]string `json:"entries"`
}

func NewImportExport(authorizer systemPermissionAuthorizer, catalog ports.CatalogRepository, updates ports.CatalogUpdateRepository, transactions database.Transactor, auditor CatalogAuditor) (*ImportExport, error) {
	if authorizer == nil || catalog == nil || updates == nil {
		return nil, ErrInvalidImportExport
	}
	read, err := authorizationdomain.ParsePermissionKey("localization.translations.read.system")
	if err != nil {
		return nil, ErrInvalidImportExport
	}
	write, err := authorizationdomain.ParsePermissionKey("localization.translations.update.system")
	if err != nil {
		return nil, ErrInvalidImportExport
	}
	return &ImportExport{authorizer: authorizer, catalog: catalog, updates: updates, transactions: transactions, auditor: auditor, readPermission: read, writePermission: write}, nil
}

func (service *ImportExport) Export(ctx context.Context, actor, locale, category string) (ExportResult, error) {
	if err := service.authorize(ctx, actor, service.readPermission); err != nil {
		return ExportResult{}, err
	}
	locale = strings.TrimSpace(locale)
	category = strings.TrimSpace(category)
	if locale == "" || category == "" {
		return ExportResult{}, ErrInvalidImportExport
	}
	found, err := service.catalog.FindEnabledLocaleByTag(ctx, locale)
	if err != nil {
		return ExportResult{}, err
	}
	overrides, err := service.catalog.ListActiveOverrides(ctx, found.ID, category)
	if err != nil {
		return ExportResult{}, err
	}
	entries := make(map[string]string, len(overrides))
	var version int64
	for _, entry := range overrides {
		entries[entry.Key] = entry.Value
		if entry.Version > version {
			version = entry.Version
		}
	}
	return ExportResult{Locale: locale, Category: category, Version: version, Entries: entries}, nil
}

func (service *ImportExport) Import(ctx context.Context, actor, locale, category string, entries map[string]string, expectedVersion int64, requestID string) (int64, error) {
	if err := service.authorize(ctx, actor, service.writePermission); err != nil {
		return 0, err
	}
	if strings.TrimSpace(locale) == "" || strings.TrimSpace(category) == "" || expectedVersion < 0 {
		return 0, ErrInvalidImportExport
	}
	found, err := service.catalog.FindEnabledLocaleByTag(ctx, locale)
	if err != nil {
		return 0, err
	}
	var version int64
	operation := func(operationContext context.Context) error {
		var updateErr error
		version, updateErr = service.updates.ReplaceCategory(operationContext, found.ID, category, entries, expectedVersion, "import")
		if updateErr != nil || service.auditor == nil {
			return updateErr
		}
		return service.auditor.RecordCatalogUpdate(operationContext, CatalogAuditEvent{ActorUserID: actor, RequestID: requestID, Locale: locale, Category: category, Version: version, EntryCount: len(entries)})
	}
	if service.transactions != nil {
		err = service.transactions.WithinTransaction(ctx, operation)
	} else {
		err = operation(ctx)
	}
	return version, err
}

func (service *ImportExport) authorize(ctx context.Context, actor string, permission authorizationdomain.PermissionKey) error {
	if service == nil || ctx == nil || strings.TrimSpace(actor) == "" {
		return ErrInvalidImportExport
	}
	allowed, err := service.authorizer.Allowed(ctx, actor, permission)
	if err != nil {
		return err
	}
	if !allowed {
		return ErrCatalogUpdateDenied
	}
	return nil
}
