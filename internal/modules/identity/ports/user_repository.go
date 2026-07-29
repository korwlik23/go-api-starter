package ports

import (
	"context"
	"errors"

	"github.com/korwlik23/go-api-starter/internal/modules/identity/domain"
)

var ErrUserNotFound = errors.New("user not found")

type UserRepository interface {
	FindByNormalizedEmail(ctx context.Context, email string) (domain.User, error)
}

type UserByIDReader interface {
	FindByID(ctx context.Context, userID string) (domain.User, error)
}
