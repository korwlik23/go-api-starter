package ports

import (
	"context"
	"errors"
)

var ErrPasswordNotFound = errors.New("password credential not found")

type PasswordRepository interface {
	FindPasswordHash(ctx context.Context, userID string) (string, error)
}

type PasswordVerifier interface {
	Verify(password, encodedHash string) (bool, error)
}
