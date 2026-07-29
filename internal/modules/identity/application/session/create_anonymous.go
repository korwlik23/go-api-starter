package session

import (
	"context"
	"errors"
	"time"

	identitydomain "github.com/korwlik23/go-api-starter/internal/modules/identity/domain"
	"github.com/korwlik23/go-api-starter/internal/modules/identity/ports"
	platformcrypto "github.com/korwlik23/go-api-starter/internal/platform/crypto"
)

var ErrInvalidAnonymousSessionPolicy = errors.New("invalid anonymous session policy")

type tokenGenerator interface {
	Generate() (platformcrypto.OpaqueToken, error)
}

type CreateAnonymousDependencies struct {
	Repository    ports.SessionRepository
	SessionTokens tokenGenerator
	CSRFTokens    tokenGenerator
	NewID         func() (string, error)
	Now           func() time.Time
	IdleTTL       time.Duration
	AbsoluteTTL   time.Duration
}

type CreateAnonymousResult struct {
	SessionToken string
	CSRFToken    string
}

type CreateAnonymous struct {
	dependencies CreateAnonymousDependencies
}

func NewCreateAnonymous(dependencies CreateAnonymousDependencies) (*CreateAnonymous, error) {
	if dependencies.Repository == nil ||
		dependencies.SessionTokens == nil ||
		dependencies.CSRFTokens == nil ||
		dependencies.NewID == nil ||
		dependencies.Now == nil ||
		dependencies.IdleTTL <= 0 ||
		dependencies.AbsoluteTTL <= 0 ||
		dependencies.IdleTTL > dependencies.AbsoluteTTL {
		return nil, ErrInvalidAnonymousSessionPolicy
	}

	return &CreateAnonymous{dependencies: dependencies}, nil
}

func (useCase *CreateAnonymous) Execute(ctx context.Context) (CreateAnonymousResult, error) {
	sessionToken, err := useCase.dependencies.SessionTokens.Generate()
	if err != nil {
		return CreateAnonymousResult{}, err
	}
	csrfToken, err := useCase.dependencies.CSRFTokens.Generate()
	if err != nil {
		return CreateAnonymousResult{}, err
	}
	sessionID, err := useCase.dependencies.NewID()
	if err != nil {
		return CreateAnonymousResult{}, err
	}

	now := useCase.dependencies.Now()
	session := identitydomain.Session{
		ID:                sessionID,
		TokenHash:         identitydomain.TokenHash(sessionToken.Hash),
		CSRFTokenHash:     identitydomain.TokenHash(csrfToken.Hash),
		State:             identitydomain.SessionStateAnonymous,
		IdleExpiresAt:     now.Add(useCase.dependencies.IdleTTL),
		AbsoluteExpiresAt: now.Add(useCase.dependencies.AbsoluteTTL),
		CreatedAt:         now,
	}
	if err := useCase.dependencies.Repository.Create(ctx, session); err != nil {
		return CreateAnonymousResult{}, err
	}

	return CreateAnonymousResult{
		SessionToken: sessionToken.Raw,
		CSRFToken:    csrfToken.Raw,
	}, nil
}
