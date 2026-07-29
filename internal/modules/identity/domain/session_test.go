package domain

import (
	"reflect"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestSessionStoresOnlyTokenHash(t *testing.T) {
	hashBytes := [32]byte{1, 2, 3}
	session := Session{
		ID:        "session-id",
		TokenHash: TokenHash(hashBytes),
	}

	require.Equal(t, TokenHash(hashBytes), session.TokenHash)

	sessionType := reflect.TypeOf(session)
	for index := 0; index < sessionType.NumField(); index++ {
		require.NotEqual(t, "RawToken", sessionType.Field(index).Name)
	}
}

func TestTokenHashRequiresSHA256Length(t *testing.T) {
	hash, err := NewTokenHash(make([]byte, 32))
	require.NoError(t, err)
	require.Equal(t, TokenHash{}, hash)

	_, err = NewTokenHash(make([]byte, 31))
	require.ErrorIs(t, err, ErrInvalidTokenHash)
}

func TestSessionValidateAtAcceptsActiveSession(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.July, 30, 2, 0, 0, 0, time.UTC)
	session := Session{
		IdleExpiresAt:     now.Add(time.Minute),
		AbsoluteExpiresAt: now.Add(time.Hour),
	}

	require.NoError(t, session.ValidateAt(now))
}

func TestSessionValidateAtRejectsRevokedSession(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.July, 30, 2, 0, 0, 0, time.UTC)
	revokedAt := now.Add(-time.Second)
	session := Session{
		RevokedAt:         &revokedAt,
		IdleExpiresAt:     now.Add(time.Minute),
		AbsoluteExpiresAt: now.Add(time.Hour),
	}

	require.ErrorIs(t, session.ValidateAt(now), ErrSessionRevoked)
}

func TestSessionValidateAtRejectsIdleExpiryAtBoundary(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.July, 30, 2, 0, 0, 0, time.UTC)
	session := Session{
		IdleExpiresAt:     now,
		AbsoluteExpiresAt: now.Add(time.Hour),
	}

	require.ErrorIs(t, session.ValidateAt(now), ErrSessionIdleExpired)
}

func TestSessionValidateAtRejectsAbsoluteExpiryAtBoundary(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.July, 30, 2, 0, 0, 0, time.UTC)
	session := Session{
		IdleExpiresAt:     now.Add(time.Minute),
		AbsoluteExpiresAt: now,
	}

	require.ErrorIs(t, session.ValidateAt(now), ErrSessionAbsoluteExpired)
}
