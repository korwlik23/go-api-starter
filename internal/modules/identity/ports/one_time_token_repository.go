package ports

import (
	"context"

	"github.com/korwlik23/go-api-starter/internal/modules/identity/domain"
)

type OneTimeTokenIssuer interface {
	Issue(
		ctx context.Context,
		token domain.OneTimeToken,
		delivery domain.EncryptedTokenDelivery,
	) error
}
