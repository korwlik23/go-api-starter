package application

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestChangePasswordVerifiesCurrentPasswordAndInvalidatesSessionsAtomically(t *testing.T) {
	now := time.Date(2026, 8, 4, 12, 0, 0, 0, time.UTC)
	reader := passwordReaderStub{hash: "current-hash"}
	verifier := passwordVerifierStub{matches: true}
	writer := &passwordChangeWriterStub{}
	service, err := NewChangePasswordService(ChangePasswordDependencies{
		Passwords: reader, Writer: writer, Verifier: verifier, Hasher: passwordHasherStub{}, Now: func() time.Time { return now },
	})
	require.NoError(t, err)

	err = service.Execute(context.Background(), "user-1", "current-password", "new-password")

	require.NoError(t, err)
	require.Equal(t, "user-1", writer.userID)
	require.Equal(t, "new-password-hash", writer.passwordHash)
	require.Equal(t, now, writer.changedAt)
}

func TestChangePasswordDoesNotWriteWhenCurrentPasswordDoesNotMatch(t *testing.T) {
	writer := &passwordChangeWriterStub{}
	service, err := NewChangePasswordService(ChangePasswordDependencies{
		Passwords: passwordReaderStub{hash: "current-hash"}, Writer: writer,
		Verifier: passwordVerifierStub{matches: false}, Hasher: passwordHasherStub{}, Now: time.Now,
	})
	require.NoError(t, err)

	err = service.Execute(context.Background(), "user-1", "wrong-password", "new-password")

	require.ErrorIs(t, err, ErrCurrentPasswordInvalid)
	require.Empty(t, writer.userID)
}

func TestChangePasswordRejectsInvalidCommandBeforeRepositoryLookup(t *testing.T) {
	service, err := NewChangePasswordService(ChangePasswordDependencies{
		Passwords: passwordReaderStub{hash: "current-hash"}, Writer: &passwordChangeWriterStub{},
		Verifier: passwordVerifierStub{matches: true}, Hasher: passwordHasherStub{}, Now: time.Now,
	})
	require.NoError(t, err)

	err = service.Execute(context.Background(), "", "current-password", "new-password")

	require.ErrorIs(t, err, ErrInvalidPasswordChange)
}

type passwordReaderStub struct{ hash string }

func (stub passwordReaderStub) FindPasswordHash(context.Context, string) (string, error) {
	if stub.hash == "" {
		return "", errors.New("missing hash")
	}
	return stub.hash, nil
}

type passwordVerifierStub struct{ matches bool }

func (stub passwordVerifierStub) Verify(string, string) (bool, error) { return stub.matches, nil }

type passwordHasherStub struct{}

func (passwordHasherStub) Hash(string) (string, error) { return "new-password-hash", nil }

type passwordChangeWriterStub struct {
	userID       string
	passwordHash string
	changedAt    time.Time
}

func (stub *passwordChangeWriterStub) ChangePasswordAndInvalidate(_ context.Context, userID, passwordHash string, changedAt time.Time) error {
	stub.userID, stub.passwordHash, stub.changedAt = userID, passwordHash, changedAt
	return nil
}
