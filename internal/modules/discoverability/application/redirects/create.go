package redirects

import (
	"context"
	"strings"

	discoverabilitydomain "github.com/korwlik23/go-api-starter/internal/modules/discoverability/domain"
	"github.com/korwlik23/go-api-starter/internal/modules/discoverability/ports"
)

func (service *Service) Create(ctx context.Context, input CreateInput) (result discoverabilitydomain.Redirect, resultErr error) {
	if service == nil || ctx == nil || strings.TrimSpace(input.AccountID) == "" || strings.TrimSpace(input.ActorUserID) == "" || strings.TrimSpace(input.RequestID) == "" || !validRedirectIdentity(input.AccountID, input.LocaleID) {
		return discoverabilitydomain.Redirect{}, ErrInvalidRedirectMutation
	}
	now := service.dependencies.Now().UTC()
	if now.IsZero() {
		return discoverabilitydomain.Redirect{}, ErrInvalidRedirectMutation
	}
	id, err := service.dependencies.NewID()
	if err != nil {
		return discoverabilitydomain.Redirect{}, err
	}
	redirect, err := discoverabilitydomain.NewRedirect(discoverabilitydomain.RedirectInput{ID: id, AccountID: input.AccountID, LocaleID: input.LocaleID, SourcePath: input.SourcePath, DestinationPath: input.DestinationPath, StatusCode: input.StatusCode, Enabled: input.Enabled, Version: 1, CreatedAt: now, UpdatedAt: now})
	if err != nil {
		return discoverabilitydomain.Redirect{}, ErrInvalidRedirectMutation
	}
	operation := func(operationContext context.Context) error {
		result, err = service.dependencies.Repository.Create(operationContext, ports.CreateRedirectCommand{Redirect: redirect})
		if err != nil {
			return err
		}
		return service.dependencies.Auditor.RecordRedirectChange(operationContext, RedirectAuditEvent{ActorUserID: input.ActorUserID, AccountID: input.AccountID, RequestID: input.RequestID, Action: "discoverability.redirect.created", RedirectID: redirect.ID, Version: redirect.Version, Data: map[string]any{"source_path": redirect.SourcePath, "status_code": int(redirect.StatusCode)}})
	}
	resultErr = service.runMutation(ctx, operation)
	return result, resultErr
}

func validRedirectIdentity(accountID, localeID string) bool {
	return ports.ValidateRedirectListQuery(ports.RedirectListQuery{AccountID: accountID, LocaleID: localeID, Limit: 1}) == nil
}
