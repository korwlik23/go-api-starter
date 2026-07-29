package session

import (
	"context"
	"errors"
	"testing"
	"time"

	identitydomain "github.com/korwlik23/go-api-starter/internal/modules/identity/domain"
	"github.com/stretchr/testify/require"
)

func TestRevokeHashesRawTokenAndRevokesPersistedSession(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.July, 30, 2, 0, 0, 0, time.UTC)
	expectedHash := [32]byte{0x42}
	hasher := &sessionTokenHasherStub{hash: expectedHash}
	repository := &sessionRevokerSpy{revoked: true}
	useCase, err := NewRevoke(RevokeDependencies{
		Repository: repository,
		Tokens:     hasher,
		Now:        func() time.Time { return now },
	})
	require.NoError(t, err)

	err = useCase.Execute(context.Background(), "raw-session-token")

	require.NoError(t, err)
	require.Equal(t, "raw-session-token", hasher.raw)
	require.Equal(t, identitydomain.TokenHash(expectedHash), repository.hash)
	require.Equal(t, now, repository.revokedAt)
	require.Equal(t, 1, repository.calls)
}

func TestRevokeIsIdempotentWhenSessionWasAlreadyRevoked(t *testing.T) {
	t.Parallel()

	useCase, err := NewRevoke(RevokeDependencies{
		Repository: &sessionRevokerSpy{revoked: false},
		Tokens:     &sessionTokenHasherStub{hash: [32]byte{0x42}},
		Now:        time.Now,
	})
	require.NoError(t, err)

	require.NoError(t, useCase.Execute(context.Background(), "already-revoked"))
}

func TestRevokeRejectsMissingTokenWithoutCallingRepository(t *testing.T) {
	t.Parallel()

	repository := &sessionRevokerSpy{}
	useCase, err := NewRevoke(RevokeDependencies{
		Repository: repository,
		Tokens:     &sessionTokenHasherStub{hash: [32]byte{0x42}},
		Now:        time.Now,
	})
	require.NoError(t, err)

	err = useCase.Execute(context.Background(), " ")

	require.ErrorIs(t, err, ErrMissingSessionToken)
	require.Zero(t, repository.calls)
}

func TestRevokeReturnsRepositoryFailure(t *testing.T) {
	t.Parallel()

	persistenceFailure := errors.New("database unavailable")
	useCase, err := NewRevoke(RevokeDependencies{
		Repository: &sessionRevokerSpy{err: persistenceFailure},
		Tokens:     &sessionTokenHasherStub{hash: [32]byte{0x42}},
		Now:        time.Now,
	})
	require.NoError(t, err)

	require.ErrorIs(
		t,
		useCase.Execute(context.Background(), "raw-session-token"),
		persistenceFailure,
	)
}

type sessionTokenHasherStub struct {
	hash [32]byte
	raw  string
}

func (stub *sessionTokenHasherStub) Hash(raw string) [32]byte {
	stub.raw = raw
	return stub.hash
}

type sessionRevokerSpy struct {
	hash      identitydomain.TokenHash
	revokedAt time.Time
	revoked   bool
	err       error
	calls     int
}

func (spy *sessionRevokerSpy) RevokeByTokenHash(
	_ context.Context,
	hash identitydomain.TokenHash,
	revokedAt time.Time,
) (bool, error) {
	spy.calls++
	spy.hash = hash
	spy.revokedAt = revokedAt
	return spy.revoked, spy.err
}
