package catalog

import (
	"context"
	"errors"

	authorizationdomain "github.com/korwlik23/go-api-starter/internal/modules/authorization/domain"
	"github.com/korwlik23/go-api-starter/internal/modules/localization/ports"
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
}

type Update struct {
	authorizer systemPermissionAuthorizer
	catalog    ports.CatalogRepository
	updates    ports.CatalogUpdateRepository
	permission authorizationdomain.PermissionKey
}

func NewUpdate(
	authorizer systemPermissionAuthorizer,
	catalog ports.CatalogRepository,
	updates ports.CatalogUpdateRepository,
) (*Update, error) {
	permission, err := authorizationdomain.ParsePermissionKey(
		"localization.translations.update.system",
	)
	if err != nil || authorizer == nil || catalog == nil || updates == nil {
		return nil, ErrInvalidCatalog
	}
	return &Update{
		authorizer: authorizer,
		catalog:    catalog,
		updates:    updates,
		permission: permission,
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
	return useCase.updates.ReplaceCategory(
		ctx,
		locale.ID,
		command.Category,
		command.Entries,
		command.ExpectedVersion,
		"editor",
	)
}
