package menus

import (
	"context"
	"strings"

	navigationdomain "github.com/korwlik23/go-api-starter/internal/modules/navigation/domain"
	"github.com/korwlik23/go-api-starter/internal/modules/navigation/ports"
)

func (service *Service) Update(ctx context.Context, input UpdateInput) (result navigationdomain.Menu, resultErr error) {
	if service == nil || ctx == nil || strings.TrimSpace(input.AccountID) == "" || strings.TrimSpace(input.ActorUserID) == "" || strings.TrimSpace(input.RequestID) == "" || input.ExpectedVersion < 1 || ports.ValidateMenuIdentity(input.AccountID, input.MenuID) != nil {
		return navigationdomain.Menu{}, ErrInvalidMenuMutation
	}
	now := service.dependencies.Now().UTC()
	if now.IsZero() {
		return navigationdomain.Menu{}, ErrInvalidMenuMutation
	}
	menu, items, err := service.buildMenu(input.MenuID, input.AccountID, input.LocaleID, input.Key, input.Name, input.Enabled, input.ExpectedVersion+1, input.Items, now)
	if err != nil {
		return navigationdomain.Menu{}, err
	}
	operation := func(operationContext context.Context) error {
		result, err = service.dependencies.Repository.Update(operationContext, ports.UpdateMenuCommand{AccountID: input.AccountID, MenuID: input.MenuID, ExpectedVersion: input.ExpectedVersion, Menu: menu, Items: items})
		if err != nil {
			return err
		}
		return service.dependencies.Auditor.RecordMenuChange(operationContext, MenuAuditEvent{ActorUserID: input.ActorUserID, AccountID: input.AccountID, RequestID: input.RequestID, Action: "navigation.menu.updated", MenuID: input.MenuID, Version: menu.Version, Data: map[string]any{"key": menu.Key, "item_count": len(items)}})
	}
	resultErr = service.runMutation(ctx, operation)
	return result, resultErr
}
