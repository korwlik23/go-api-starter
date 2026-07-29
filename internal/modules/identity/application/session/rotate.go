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
	Repository            ports.SessionRotator
	SessionTokens         tokenGenerator
	CSRFTokens            tokenGenerator
	NewID                 func() (string, error)
	Now                   func() time.Time
	IdleTTL               time.Duration
	AbsoluteTTL           time.Duration
	MFAPendingIdleTTL     time.Duration
	MFAPendingAbsoluteTTL time.Duration
}

type RotationResult struct {
	SessionID    string
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
		dependencies.IdleTTL > dependencies.AbsoluteTTL ||
		dependencies.MFAPendingIdleTTL <= 0 ||
		dependencies.MFAPendingAbsoluteTTL <= 0 ||
		dependencies.MFAPendingIdleTTL > dependencies.MFAPendingAbsoluteTTL {
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
	return rotator.rotate(
		ctx,
		currentHash,
		userID,
		authorizationVersion,
		identitydomain.SessionStateAuthenticated,
		rotator.dependencies.IdleTTL,
		rotator.dependencies.AbsoluteTTL,
	)
}

func (rotator *Rotator) MFAPending(
	ctx context.Context,
	currentHash identitydomain.TokenHash,
	userID string,
	authorizationVersion int64,
) (RotationResult, error) {
	return rotator.rotate(
		ctx,
		currentHash,
		userID,
		authorizationVersion,
		identitydomain.SessionStateMFAPending,
		rotator.dependencies.MFAPendingIdleTTL,
		rotator.dependencies.MFAPendingAbsoluteTTL,
	)
}

func (rotator *Rotator) rotate(
	ctx context.Context,
	currentHash identitydomain.TokenHash,
	userID string,
	authorizationVersion int64,
	state identitydomain.SessionState,
	idleTTL time.Duration,
	absoluteTTL time.Duration,
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
		State:                state,
		UserID:               stringPointerValue(userID),
		AuthorizationVersion: int64PointerValue(authorizationVersion),
		IdleExpiresAt:        now.Add(idleTTL),
		AbsoluteExpiresAt:    now.Add(absoluteTTL),
		CreatedAt:            now,
	}
	if err := rotator.dependencies.Repository.Rotate(ctx, currentHash, replacement); err != nil {
		return RotationResult{}, err
	}

	return RotationResult{
		SessionID:    sessionID,
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
