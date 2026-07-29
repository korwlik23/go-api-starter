package session

import (
	"context"
	"testing"
	"time"

	identitydomain "github.com/korwlik23/go-api-starter/internal/modules/identity/domain"
	"github.com/korwlik23/go-api-starter/internal/modules/identity/ports"
	platformcrypto "github.com/korwlik23/go-api-starter/internal/platform/crypto"
	"github.com/stretchr/testify/require"
)

func TestRotateAuthenticatedReplacesPreAuthSession(t *testing.T) {
	now := time.Date(2026, time.July, 30, 6, 0, 0, 0, time.UTC)
	currentHash := identitydomain.TokenHash{0x01}
	sessionHash := [32]byte{0x02}
	csrfHash := [32]byte{0x03}
	repository := &sessionRotatorSpy{}
	rotator, err := NewRotator(RotatorDependencies{
		Repository: repository,
		SessionTokens: tokenGeneratorStub{token: platformcrypto.OpaqueToken{
			Raw: "new-session-token", Hash: sessionHash,
		}},
		CSRFTokens: tokenGeneratorStub{token: platformcrypto.OpaqueToken{
			Raw: "new-csrf-token", Hash: csrfHash,
		}},
		NewID:       func() (string, error) { return "new-session-id", nil },
		Now:         func() time.Time { return now },
		IdleTTL:     30 * time.Minute,
		AbsoluteTTL: 12 * time.Hour,
	})
	require.NoError(t, err)

	result, err := rotator.Authenticated(
		context.Background(),
		currentHash,
		"user-id",
		7,
	)
	require.NoError(t, err)
	require.Equal(t, "new-session-token", result.SessionToken)
	require.Equal(t, "new-csrf-token", result.CSRFToken)
	require.Equal(t, currentHash, repository.currentHash)
	require.Equal(t, identitydomain.Session{
		ID:                   "new-session-id",
		TokenHash:            identitydomain.TokenHash(sessionHash),
		CSRFTokenHash:        identitydomain.TokenHash(csrfHash),
		State:                identitydomain.SessionStateAuthenticated,
		UserID:               stringPointer("user-id"),
		AuthorizationVersion: int64Pointer(7),
		IdleExpiresAt:        now.Add(30 * time.Minute),
		AbsoluteExpiresAt:    now.Add(12 * time.Hour),
		CreatedAt:            now,
	}, repository.replacement)
	require.NotEqual(t, currentHash, repository.replacement.TokenHash)
}

func TestRotateAuthenticatedReturnsNoTokensWhenConditionalRevokeLoses(t *testing.T) {
	repository := &sessionRotatorSpy{err: ports.ErrSessionRotationConflict}
	rotator, err := NewRotator(RotatorDependencies{
		Repository:    repository,
		SessionTokens: tokenGeneratorStub{token: platformcrypto.OpaqueToken{Raw: "session"}},
		CSRFTokens:    tokenGeneratorStub{token: platformcrypto.OpaqueToken{Raw: "csrf"}},
		NewID:         func() (string, error) { return "new-session-id", nil },
		Now:           time.Now,
		IdleTTL:       time.Minute,
		AbsoluteTTL:   2 * time.Minute,
	})
	require.NoError(t, err)

	result, err := rotator.Authenticated(
		context.Background(),
		identitydomain.TokenHash{1},
		"user-id",
		1,
	)
	require.ErrorIs(t, err, ports.ErrSessionRotationConflict)
	require.Empty(t, result)
}

func TestRotateAuthenticatedValidatesPrincipalSnapshot(t *testing.T) {
	rotator, err := NewRotator(RotatorDependencies{
		Repository:    &sessionRotatorSpy{},
		SessionTokens: tokenGeneratorStub{},
		CSRFTokens:    tokenGeneratorStub{},
		NewID:         func() (string, error) { return "new-session-id", nil },
		Now:           time.Now,
		IdleTTL:       time.Minute,
		AbsoluteTTL:   2 * time.Minute,
	})
	require.NoError(t, err)

	_, err = rotator.Authenticated(
		context.Background(),
		identitydomain.TokenHash{1},
		"",
		0,
	)
	require.ErrorIs(t, err, ErrInvalidSessionRotation)
}

type sessionRotatorSpy struct {
	currentHash identitydomain.TokenHash
	replacement identitydomain.Session
	err         error
}

func (repository *sessionRotatorSpy) Rotate(
	_ context.Context,
	currentHash identitydomain.TokenHash,
	replacement identitydomain.Session,
) error {
	repository.currentHash = currentHash
	repository.replacement = replacement
	return repository.err
}

func stringPointer(value string) *string {
	return &value
}

func int64Pointer(value int64) *int64 {
	return &value
}
