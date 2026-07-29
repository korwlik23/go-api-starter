package ports

import (
	"context"
	"errors"
	"time"

	"github.com/korwlik23/go-api-starter/internal/modules/identity/domain"
)

var (
	ErrSessionNotFound   = errors.New("session not found")
	ErrInvalidPurgeLimit = errors.New("session purge limit must be between 1 and 10000")
)

type SessionCreator interface {
	Create(ctx context.Context, session domain.Session) error
}

type SessionRepository interface {
	SessionCreator
	FindByTokenHash(ctx context.Context, hash domain.TokenHash) (domain.Session, error)
	RevokeByTokenHash(ctx context.Context, hash domain.TokenHash, revokedAt time.Time) (bool, error)
	PurgeExpired(ctx context.Context, now time.Time, limit int) (int64, error)
}
