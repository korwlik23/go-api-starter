package menus

import (
	"context"
	"strings"

	navigationdomain "github.com/korwlik23/go-api-starter/internal/modules/navigation/domain"
	"github.com/korwlik23/go-api-starter/internal/modules/navigation/ports"
)

func (service *Service) Create(ctx context.Context, input CreateInput) (result navigationdomain.Menu, resultErr error) {
	if service == nil || ctx == nil || strings.TrimSpace(input.AccountID) == "" || strings.TrimSpace(input.ActorUserID) == "" || strings.TrimSpace(input.RequestID) == "" || strings.TrimSpace(input.LocaleID) == "" {
		return navigationdomain.Menu{}, ErrInvalidMenuMutation
	}
	now := service.dependencies.Now().UTC()
	if now.IsZero() || ports.ValidateMenuIdentity(input.AccountID, input.AccountID) != nil {
		return navigationdomain.Menu{}, ErrInvalidMenuMutation
	}
	id, err := service.dependencies.NewID()
	if err != nil {
		return navigationdomain.Menu{}, err
	}
	menu, items, err := service.buildMenu(id, input.AccountID, input.LocaleID, input.Key, input.Name, input.Enabled, 1, input.Items, now)
	if err != nil {
		return navigationdomain.Menu{}, err
	}
	operation := func(operationContext context.Context) error {
		result, err = service.dependencies.Repository.Create(operationContext, ports.CreateMenuCommand{Menu: menu, Items: items})
		if err != nil {
			return err
		}
		return service.dependencies.Auditor.RecordMenuChange(operationContext, MenuAuditEvent{
			ActorUserID: input.ActorUserID, AccountID: input.AccountID, RequestID: input.RequestID,
			Action: "navigation.menu.created", MenuID: menu.ID, Version: menu.Version,
			Data: map[string]any{"key": menu.Key, "item_count": len(items)},
		})
	}
	resultErr = service.runMutation(ctx, operation)
	return result, resultErr
}
