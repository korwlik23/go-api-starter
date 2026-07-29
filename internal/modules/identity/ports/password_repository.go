package ports

import (
	"context"
	"errors"
	"time"
)

var ErrPasswordNotFound = errors.New("password credential not found")

type PasswordRepository interface {
	FindPasswordHash(ctx context.Context, userID string) (string, error)
}

type PasswordWriter interface {
	ChangePassword(
		ctx context.Context,
		userID string,
		passwordHash string,
		changedAt time.Time,
	) error
	RehashPassword(ctx context.Context, userID string, passwordHash string) error
}

type PasswordVerifier interface {
	Verify(password, encodedHash string) (bool, error)
}
