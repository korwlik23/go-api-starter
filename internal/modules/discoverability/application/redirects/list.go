package redirects

import (
	"context"
	"github.com/korwlik23/go-api-starter/internal/modules/discoverability/ports"
)

func (service *Service) List(ctx context.Context, input ListInput) (ports.RedirectListPage, error) {
	if service == nil || ctx == nil || ports.ValidateRedirectListQuery(ports.RedirectListQuery{AccountID: input.AccountID, LocaleID: input.LocaleID, Enabled: input.Enabled, After: input.After, Limit: input.Limit}) != nil {
		return ports.RedirectListPage{}, ErrInvalidRedirectMutation
	}
	return service.dependencies.Repository.List(ctx, ports.RedirectListQuery{AccountID: input.AccountID, LocaleID: input.LocaleID, Enabled: input.Enabled, After: input.After, Limit: input.Limit})
}
