package ports

import (
	"context"

	"github.com/korwlik23/go-api-starter/internal/modules/identity/domain"
)

type SessionRepository interface {
	Create(ctx context.Context, session domain.Session) error
}
