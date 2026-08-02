package redirects

import (
	"context"
	"github.com/korwlik23/go-api-starter/internal/modules/discoverability/ports"
	"strings"
)

func (service *Service) Delete(ctx context.Context, input DeleteInput) error {
	if service == nil || ctx == nil || strings.TrimSpace(input.ActorUserID) == "" || strings.TrimSpace(input.RequestID) == "" || input.ExpectedVersion < 1 || ports.ValidateRedirectIdentity(input.AccountID, input.RedirectID) != nil {
		return ErrInvalidRedirectMutation
	}
	now := service.dependencies.Now().UTC()
	if now.IsZero() {
		return ErrInvalidRedirectMutation
	}
	return service.runMutation(ctx, func(operationContext context.Context) error {
		if err := service.dependencies.Repository.Delete(operationContext, ports.DeleteRedirectCommand{AccountID: input.AccountID, RedirectID: input.RedirectID, ExpectedVersion: input.ExpectedVersion, DeletedAt: now}); err != nil {
			return err
		}
		return service.dependencies.Auditor.RecordRedirectChange(operationContext, RedirectAuditEvent{ActorUserID: input.ActorUserID, AccountID: input.AccountID, RequestID: input.RequestID, Action: "discoverability.redirect.deleted", RedirectID: input.RedirectID, Version: input.ExpectedVersion + 1, Data: map[string]any{"deleted": true}})
	})
}
