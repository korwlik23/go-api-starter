package catalog

import (
	"context"
	"errors"

	authorizationdomain "github.com/korwlik23/go-api-starter/internal/modules/authorization/domain"
	"github.com/korwlik23/go-api-starter/internal/modules/localization/ports"
	"github.com/korwlik23/go-api-starter/internal/platform/database"
)

var ErrCatalogUpdateDenied = errors.New("catalog update denied")

type systemPermissionAuthorizer interface {
	Allowed(
		ctx context.Context,
		userID string,
		permission authorizationdomain.PermissionKey,
	) (bool, error)
}

type UpdateCommand struct {
	Locale          string
	Category        string
	Entries         map[string]string
	ExpectedVersion int64
	RequestID       string
}

type CatalogAuditEvent struct {
	ActorUserID string
	RequestID   string
	Locale      string
	Category    string
	Version     int64
	EntryCount  int
}

type CatalogAuditor interface {
	RecordCatalogUpdate(context.Context, CatalogAuditEvent) error
}

type Update struct {
	authorizer   systemPermissionAuthorizer
	catalog      ports.CatalogRepository
	updates      ports.CatalogUpdateRepository
	permission   authorizationdomain.PermissionKey
	transactions database.Transactor
	auditor      CatalogAuditor
}

func NewUpdate(
	authorizer systemPermissionAuthorizer,
	catalog ports.CatalogRepository,
	updates ports.CatalogUpdateRepository,
) (*Update, error) {
	return NewUpdateWithDependencies(authorizer, catalog, updates, nil, nil)
}

func NewUpdateWithDependencies(
	authorizer systemPermissionAuthorizer,
	catalog ports.CatalogRepository,
	updates ports.CatalogUpdateRepository,
	transactions database.Transactor,
	auditor CatalogAuditor,
) (*Update, error) {
	permission, err := authorizationdomain.ParsePermissionKey(
		"localization.translations.update.system",
	)
	if err != nil || authorizer == nil || catalog == nil || updates == nil {
		return nil, ErrInvalidCatalog
	}
	return &Update{
		authorizer:   authorizer,
		catalog:      catalog,
		updates:      updates,
		permission:   permission,
		transactions: transactions,
		auditor:      auditor,
	}, nil
}

func (useCase *Update) Execute(
	ctx context.Context,
	authenticatedUserID string,
	command UpdateCommand,
) (int64, error) {
	allowed, err := useCase.authorizer.Allowed(
		ctx,
		authenticatedUserID,
		useCase.permission,
	)
	if err != nil {
		return 0, err
	}
	if !allowed {
		return 0, ErrCatalogUpdateDenied
	}
	locale, err := useCase.catalog.FindEnabledLocaleByTag(
		ctx,
		command.Locale,
	)
	if err != nil {
		return 0, err
	}
	var version int64
	operation := func(operationContext context.Context) error {
		var updateErr error
		version, updateErr = useCase.updates.ReplaceCategory(
			operationContext, locale.ID, command.Category, command.Entries,
			command.ExpectedVersion, "editor",
		)
		if updateErr != nil || useCase.auditor == nil {
			return updateErr
		}
		return useCase.auditor.RecordCatalogUpdate(operationContext, CatalogAuditEvent{
			ActorUserID: authenticatedUserID, RequestID: command.RequestID,
			Locale: command.Locale, Category: command.Category,
			Version: version, EntryCount: len(command.Entries),
		})
	}
	if useCase.transactions != nil {
		err = useCase.transactions.WithinTransaction(ctx, operation)
	} else {
		err = operation(ctx)
	}
	return version, err
}
