package ports

import (
	"context"
	"time"

	"github.com/korwlik23/go-api-starter/internal/modules/identity/domain"
)

type SessionManagementRepository interface {
	ListActiveByUser(
		ctx context.Context,
		userID string,
		now time.Time,
	) ([]domain.Session, error)
	RevokeOwnedByID(
		ctx context.Context,
		userID string,
		sessionID string,
		revokedAt time.Time,
	) (bool, error)
}
