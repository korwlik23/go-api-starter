package menus

import (
	"context"
	"strings"

	"github.com/korwlik23/go-api-starter/internal/modules/navigation/ports"
)

func (service *Service) Delete(ctx context.Context, input DeleteInput) (resultErr error) {
	if service == nil || ctx == nil || strings.TrimSpace(input.AccountID) == "" || strings.TrimSpace(input.ActorUserID) == "" || strings.TrimSpace(input.RequestID) == "" || input.ExpectedVersion < 1 || ports.ValidateMenuIdentity(input.AccountID, input.MenuID) != nil {
		return ErrInvalidMenuMutation
	}
	now := service.dependencies.Now().UTC()
	if now.IsZero() {
		return ErrInvalidMenuMutation
	}
	operation := func(operationContext context.Context) error {
		if err := service.dependencies.Repository.Delete(operationContext, ports.DeleteMenuCommand{AccountID: input.AccountID, MenuID: input.MenuID, ExpectedVersion: input.ExpectedVersion, DeletedAt: now}); err != nil {
			return err
		}
		return service.dependencies.Auditor.RecordMenuChange(operationContext, MenuAuditEvent{ActorUserID: input.ActorUserID, AccountID: input.AccountID, RequestID: input.RequestID, Action: "navigation.menu.deleted", MenuID: input.MenuID, Version: input.ExpectedVersion + 1, Data: map[string]any{"deleted": true}})
	}
	return service.runMutation(ctx, operation)
}
