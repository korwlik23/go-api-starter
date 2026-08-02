package redirects

import (
	"context"
	"strings"

	discoverabilitydomain "github.com/korwlik23/go-api-starter/internal/modules/discoverability/domain"
	"github.com/korwlik23/go-api-starter/internal/modules/discoverability/ports"
)

func (service *Service) Update(ctx context.Context, input UpdateInput) (result discoverabilitydomain.Redirect, resultErr error) {
	if service == nil || ctx == nil || strings.TrimSpace(input.ActorUserID) == "" || strings.TrimSpace(input.RequestID) == "" || input.ExpectedVersion < 1 || ports.ValidateRedirectIdentity(input.AccountID, input.RedirectID) != nil || !validRedirectIdentity(input.AccountID, input.LocaleID) {
		return discoverabilitydomain.Redirect{}, ErrInvalidRedirectMutation
	}
	now := service.dependencies.Now().UTC()
	if now.IsZero() {
		return discoverabilitydomain.Redirect{}, ErrInvalidRedirectMutation
	}
	redirect, err := discoverabilitydomain.NewRedirect(discoverabilitydomain.RedirectInput{ID: input.RedirectID, AccountID: input.AccountID, LocaleID: input.LocaleID, SourcePath: input.SourcePath, DestinationPath: input.DestinationPath, StatusCode: input.StatusCode, Enabled: input.Enabled, Version: input.ExpectedVersion + 1, CreatedAt: now, UpdatedAt: now})
	if err != nil {
		return discoverabilitydomain.Redirect{}, ErrInvalidRedirectMutation
	}
	operation := func(operationContext context.Context) error {
		result, err = service.dependencies.Repository.Update(operationContext, ports.UpdateRedirectCommand{AccountID: input.AccountID, RedirectID: input.RedirectID, ExpectedVersion: input.ExpectedVersion, Redirect: redirect})
		if err != nil {
			return err
		}
		return service.dependencies.Auditor.RecordRedirectChange(operationContext, RedirectAuditEvent{ActorUserID: input.ActorUserID, AccountID: input.AccountID, RequestID: input.RequestID, Action: "discoverability.redirect.updated", RedirectID: input.RedirectID, Version: redirect.Version, Data: map[string]any{"source_path": redirect.SourcePath, "status_code": int(redirect.StatusCode)}})
	}
	resultErr = service.runMutation(ctx, operation)
	return result, resultErr
}
