package session

import (
	"context"
	"errors"
	"strings"
	"time"

	identitydomain "github.com/korwlik23/go-api-starter/internal/modules/identity/domain"
)

var (
	ErrInvalidSessionRevocation = errors.New("invalid session revocation dependencies")
	ErrMissingSessionToken      = errors.New("session token is required")
)

type sessionTokenHasher interface {
	Hash(raw string) [32]byte
}

type sessionRevoker interface {
	RevokeByTokenHash(
		ctx context.Context,
		hash identitydomain.TokenHash,
		revokedAt time.Time,
	) (bool, error)
}

type RevokeDependencies struct {
	Repository sessionRevoker
	Tokens     sessionTokenHasher
	Now        func() time.Time
}

type Revoke struct {
	dependencies RevokeDependencies
}

func NewRevoke(dependencies RevokeDependencies) (*Revoke, error) {
	if dependencies.Repository == nil ||
		dependencies.Tokens == nil ||
		dependencies.Now == nil {
		return nil, ErrInvalidSessionRevocation
	}
	return &Revoke{dependencies: dependencies}, nil
}

func (useCase *Revoke) Execute(ctx context.Context, rawToken string) error {
	if strings.TrimSpace(rawToken) == "" {
		return ErrMissingSessionToken
	}

	hash := identitydomain.TokenHash(useCase.dependencies.Tokens.Hash(rawToken))
	_, err := useCase.dependencies.Repository.RevokeByTokenHash(
		ctx,
		hash,
		useCase.dependencies.Now(),
	)
	return err
}
