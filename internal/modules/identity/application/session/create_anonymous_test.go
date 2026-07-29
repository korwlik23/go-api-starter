package session

import (
	"context"
	"errors"
	"testing"
	"time"

	identitydomain "github.com/korwlik23/go-api-starter/internal/modules/identity/domain"
	platformcrypto "github.com/korwlik23/go-api-starter/internal/platform/crypto"
	"github.com/stretchr/testify/require"
)

func TestCreateAnonymousSession(t *testing.T) {
	now := time.Date(2026, time.July, 30, 1, 0, 0, 0, time.UTC)
	sessionHash := [32]byte{1, 2, 3}
	csrfHash := [32]byte{4, 5, 6}
	repository := &sessionRepositorySpy{}

	useCase, err := NewCreateAnonymous(CreateAnonymousDependencies{
		Repository: repository,
		SessionTokens: tokenGeneratorStub{token: platformcrypto.OpaqueToken{
			Raw:  "raw-session-token",
			Hash: sessionHash,
		}},
		CSRFTokens: tokenGeneratorStub{token: platformcrypto.OpaqueToken{
			Raw:  "raw-csrf-token",
			Hash: csrfHash,
		}},
		NewID:       func() (string, error) { return "session-id", nil },
		Now:         func() time.Time { return now },
		IdleTTL:     15 * time.Minute,
		AbsoluteTTL: 30 * time.Minute,
	})
	require.NoError(t, err)

	result, err := useCase.Execute(context.Background())
	require.NoError(t, err)
	require.Equal(t, "raw-session-token", result.SessionToken)
	require.Equal(t, "raw-csrf-token", result.CSRFToken)
	require.Equal(t, identitydomain.Session{
		ID:                "session-id",
		TokenHash:         identitydomain.TokenHash(sessionHash),
		CSRFTokenHash:     identitydomain.TokenHash(csrfHash),
		State:             identitydomain.SessionStateAnonymous,
		IdleExpiresAt:     now.Add(15 * time.Minute),
		AbsoluteExpiresAt: now.Add(30 * time.Minute),
		CreatedAt:         now,
	}, repository.created)
	require.NotContains(t, repository.created.ID, result.SessionToken)
	require.NotContains(t, repository.created.ID, result.CSRFToken)
}

func TestCreateAnonymousSessionDoesNotReturnTokensWhenPersistenceFails(t *testing.T) {
	persistErr := errors.New("database unavailable")
	repository := &sessionRepositorySpy{createErr: persistErr}
	useCase, err := NewCreateAnonymous(CreateAnonymousDependencies{
		Repository:    repository,
		SessionTokens: tokenGeneratorStub{token: platformcrypto.OpaqueToken{Raw: "session"}},
		CSRFTokens:    tokenGeneratorStub{token: platformcrypto.OpaqueToken{Raw: "csrf"}},
		NewID:         func() (string, error) { return "session-id", nil },
		Now:           time.Now,
		IdleTTL:       time.Minute,
		AbsoluteTTL:   2 * time.Minute,
	})
	require.NoError(t, err)

	result, err := useCase.Execute(context.Background())
	require.ErrorIs(t, err, persistErr)
	require.Empty(t, result)
}

func TestCreateAnonymousSessionRejectsInvalidExpiryPolicy(t *testing.T) {
	_, err := NewCreateAnonymous(CreateAnonymousDependencies{
		Repository:    &sessionRepositorySpy{},
		SessionTokens: tokenGeneratorStub{},
		CSRFTokens:    tokenGeneratorStub{},
		NewID:         func() (string, error) { return "session-id", nil },
		Now:           time.Now,
		IdleTTL:       2 * time.Minute,
		AbsoluteTTL:   time.Minute,
	})
	require.ErrorIs(t, err, ErrInvalidAnonymousSessionPolicy)
}

type sessionRepositorySpy struct {
	created   identitydomain.Session
	createErr error
}

func (repository *sessionRepositorySpy) Create(
	_ context.Context,
	value identitydomain.Session,
) error {
	repository.created = value
	return repository.createErr
}

type tokenGeneratorStub struct {
	token platformcrypto.OpaqueToken
	err   error
}

func (generator tokenGeneratorStub) Generate() (platformcrypto.OpaqueToken, error) {
	return generator.token, generator.err
}
