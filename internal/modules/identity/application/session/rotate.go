package session

import (
	"context"
	"errors"
	"time"

	identitydomain "github.com/korwlik23/go-api-starter/internal/modules/identity/domain"
	"github.com/korwlik23/go-api-starter/internal/modules/identity/ports"
)

var ErrInvalidSessionRotation = errors.New("invalid session rotation")

type RotatorDependencies struct {
	Repository    ports.SessionRotator
	SessionTokens tokenGenerator
	CSRFTokens    tokenGenerator
	NewID         func() (string, error)
	Now           func() time.Time
	IdleTTL       time.Duration
	AbsoluteTTL   time.Duration
}

type RotationResult struct {
	SessionToken string
	CSRFToken    string
}

type Rotator struct {
	dependencies RotatorDependencies
}

func NewRotator(dependencies RotatorDependencies) (*Rotator, error) {
	if dependencies.Repository == nil ||
		dependencies.SessionTokens == nil ||
		dependencies.CSRFTokens == nil ||
		dependencies.NewID == nil ||
		dependencies.Now == nil ||
		dependencies.IdleTTL <= 0 ||
		dependencies.AbsoluteTTL <= 0 ||
		dependencies.IdleTTL > dependencies.AbsoluteTTL {
		return nil, ErrInvalidSessionRotation
	}
	return &Rotator{dependencies: dependencies}, nil
}

func (rotator *Rotator) Authenticated(
	ctx context.Context,
	currentHash identitydomain.TokenHash,
	userID string,
	authorizationVersion int64,
) (RotationResult, error) {
	if userID == "" || authorizationVersion < 1 {
		return RotationResult{}, ErrInvalidSessionRotation
	}

	sessionToken, err := rotator.dependencies.SessionTokens.Generate()
	if err != nil {
		return RotationResult{}, err
	}
	csrfToken, err := rotator.dependencies.CSRFTokens.Generate()
	if err != nil {
		return RotationResult{}, err
	}
	sessionID, err := rotator.dependencies.NewID()
	if err != nil {
		return RotationResult{}, err
	}

	now := rotator.dependencies.Now()
	replacement := identitydomain.Session{
		ID:                   sessionID,
		TokenHash:            identitydomain.TokenHash(sessionToken.Hash),
		CSRFTokenHash:        identitydomain.TokenHash(csrfToken.Hash),
		State:                identitydomain.SessionStateAuthenticated,
		UserID:               stringPointerValue(userID),
		AuthorizationVersion: int64PointerValue(authorizationVersion),
		IdleExpiresAt:        now.Add(rotator.dependencies.IdleTTL),
		AbsoluteExpiresAt:    now.Add(rotator.dependencies.AbsoluteTTL),
		CreatedAt:            now,
	}
	if err := rotator.dependencies.Repository.Rotate(ctx, currentHash, replacement); err != nil {
		return RotationResult{}, err
	}

	return RotationResult{
		SessionToken: sessionToken.Raw,
		CSRFToken:    csrfToken.Raw,
	}, nil
}

func stringPointerValue(value string) *string {
	return &value
}

func int64PointerValue(value int64) *int64 {
	return &value
}
