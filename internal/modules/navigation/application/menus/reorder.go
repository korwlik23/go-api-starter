package menus

import (
	"context"
	"strings"

	navigationdomain "github.com/korwlik23/go-api-starter/internal/modules/navigation/domain"
	"github.com/korwlik23/go-api-starter/internal/modules/navigation/ports"
)

func (service *Service) Reorder(ctx context.Context, input ReorderInput) (result navigationdomain.Menu, resultErr error) {
	if service == nil || ctx == nil || strings.TrimSpace(input.AccountID) == "" || strings.TrimSpace(input.ActorUserID) == "" || strings.TrimSpace(input.RequestID) == "" || input.ExpectedVersion < 1 || ports.ValidateMenuIdentity(input.AccountID, input.MenuID) != nil {
		return navigationdomain.Menu{}, ErrInvalidMenuMutation
	}
	now := service.dependencies.Now().UTC()
	if now.IsZero() {
		return navigationdomain.Menu{}, ErrInvalidMenuMutation
	}
	current, err := service.dependencies.Repository.Get(ctx, input.AccountID, input.MenuID)
	if err != nil {
		return navigationdomain.Menu{}, err
	}
	currentItems := navigationdomain.FlattenItems(current.Items)
	if len(currentItems) != len(input.Items) {
		return navigationdomain.Menu{}, ErrInvalidMenuMutation
	}
	byID := make(map[string]navigationdomain.MenuItem, len(currentItems))
	for _, item := range currentItems {
		byID[item.ID] = item
	}
	seen := make(map[string]struct{}, len(input.Items))
	updated := make([]navigationdomain.MenuItem, 0, len(input.Items))
	for _, reorder := range input.Items {
		item, ok := byID[reorder.ID]
		if !ok || reorder.Position < 0 {
			return navigationdomain.Menu{}, ErrInvalidMenuMutation
		}
		if _, exists := seen[reorder.ID]; exists {
			return navigationdomain.Menu{}, ErrInvalidMenuMutation
		}
		seen[reorder.ID] = struct{}{}
		item.ParentID, item.Position, item.UpdatedAt = strings.TrimSpace(reorder.ParentID), reorder.Position, now
		updated = append(updated, item)
	}
	if len(seen) != len(byID) || navigationdomain.ValidateTree(updated, navigationdomain.MaxMenuDepth) != nil {
		return navigationdomain.Menu{}, ErrInvalidMenuMutation
	}
	operation := func(operationContext context.Context) error {
		result, err = service.dependencies.Repository.Reorder(operationContext, ports.ReorderMenuCommand{AccountID: input.AccountID, MenuID: input.MenuID, ExpectedVersion: input.ExpectedVersion, Items: updated, UpdatedAt: now})
		if err != nil {
			return err
		}
		return service.dependencies.Auditor.RecordMenuChange(operationContext, MenuAuditEvent{ActorUserID: input.ActorUserID, AccountID: input.AccountID, RequestID: input.RequestID, Action: "navigation.menu.reordered", MenuID: input.MenuID, Version: input.ExpectedVersion + 1, Data: map[string]any{"item_count": len(updated)}})
	}
	resultErr = service.runMutation(ctx, operation)
	return result, resultErr
}
