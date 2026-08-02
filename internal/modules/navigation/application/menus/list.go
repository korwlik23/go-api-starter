package menus

import (
	"context"

	navigationdomain "github.com/korwlik23/go-api-starter/internal/modules/navigation/domain"
	"github.com/korwlik23/go-api-starter/internal/modules/navigation/ports"
)

func (service *Service) List(ctx context.Context, input ListInput) (ports.MenuListPage, error) {
	if service == nil || ctx == nil || ports.ValidateMenuListQuery(ports.MenuListQuery{AccountID: input.AccountID, LocaleID: input.LocaleID, Enabled: input.Enabled, After: input.After, Limit: input.Limit}) != nil {
		return ports.MenuListPage{}, ErrInvalidMenuQuery
	}
	return service.dependencies.Repository.List(ctx, ports.MenuListQuery{AccountID: input.AccountID, LocaleID: input.LocaleID, Enabled: input.Enabled, After: input.After, Limit: input.Limit})
}

func (service *Service) Get(ctx context.Context, accountID, menuID string) (navigationdomain.Menu, error) {
	if service == nil || ctx == nil || ports.ValidateMenuIdentity(accountID, menuID) != nil {
		return navigationdomain.Menu{}, ErrInvalidMenuQuery
	}
	return service.dependencies.Repository.Get(ctx, accountID, menuID)
}
