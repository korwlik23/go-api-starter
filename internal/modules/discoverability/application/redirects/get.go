package redirects

import (
	"context"

	discoverabilitydomain "github.com/korwlik23/go-api-starter/internal/modules/discoverability/domain"
	"github.com/korwlik23/go-api-starter/internal/modules/discoverability/ports"
)

func (service *Service) Get(ctx context.Context, accountID, redirectID string) (discoverabilitydomain.Redirect, error) {
	if service == nil || ctx == nil || ports.ValidateRedirectIdentity(accountID, redirectID) != nil {
		return discoverabilitydomain.Redirect{}, ports.ErrInvalidRedirectQuery
	}
	return service.dependencies.Repository.Get(ctx, accountID, redirectID)
}
