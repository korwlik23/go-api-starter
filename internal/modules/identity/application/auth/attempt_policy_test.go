package auth

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/korwlik23/go-api-starter/internal/modules/identity/ports"
	"github.com/stretchr/testify/require"
)

func TestAttemptPolicyBuildsStableSubjectThenIPKeys(t *testing.T) {
	repository := &authAttemptRepositoryStub{}
	subjectFingerprinter := &fingerprinterStub{marker: 0x11}
	ipFingerprinter := &fingerprinterStub{marker: 0x22}
	policy, err := NewAttemptPolicy(AttemptPolicyDependencies{
		Repository:           repository,
		SubjectFingerprinter: subjectFingerprinter,
		IPFingerprinter:      ipFingerprinter,
		Now:                  time.Now,
		Config: ports.AuthAttemptPolicy{
			Threshold:    5,
			Window:       15 * time.Minute,
			LockDuration: 30 * time.Minute,
		},
	})
	require.NoError(t, err)

	keys, err := policy.Check(
		context.Background(),
		" USER@Example.COM ",
		"2001:0db8:0000:0000:0000:0000:0000:0001",
	)
	require.NoError(t, err)
	require.Equal(t, []ports.AuthAttemptKey{
		{Type: ports.AuthAttemptKeySubject, Hash: [32]byte{0x11}},
		{Type: ports.AuthAttemptKeyIP, Hash: [32]byte{0x22}},
	}, keys)
	require.Equal(t, "user@example.com", subjectFingerprinter.value)
	require.Equal(t, "2001:db8::1", ipFingerprinter.value)
	require.Equal(t, keys, repository.checked)
}

func TestAttemptPolicyReturnsStableLockedOutcome(t *testing.T) {
	lockedUntil := time.Date(2026, time.July, 30, 4, 0, 0, 0, time.UTC)
	repository := &authAttemptRepositoryStub{
		decision: ports.AuthAttemptDecision{LockedUntil: &lockedUntil},
	}
	policy := attemptPolicyForTest(t, repository)

	_, err := policy.Check(context.Background(), "user@example.com", "192.0.2.1")
	require.Equal(t, ErrAuthenticationTemporarilyLocked, err)
}

func TestAttemptPolicyRecordsFailureAndResetsOnlySubjectOnSuccess(t *testing.T) {
	repository := &authAttemptRepositoryStub{}
	policy := attemptPolicyForTest(t, repository)
	keys, err := policy.Check(context.Background(), "user@example.com", "192.0.2.1")
	require.NoError(t, err)

	err = policy.RecordFailure(context.Background(), keys)
	require.NoError(t, err)
	require.Equal(t, keys, repository.failed)

	err = policy.RecordSuccess(context.Background(), keys)
	require.NoError(t, err)
	require.Equal(t, keys[0], repository.reset)
}

func TestAttemptPolicyFailsClosedOnRepositoryFailure(t *testing.T) {
	repositoryErr := errors.New("database unavailable")
	repository := &authAttemptRepositoryStub{err: repositoryErr}
	policy := attemptPolicyForTest(t, repository)

	_, err := policy.Check(context.Background(), "user@example.com", "192.0.2.1")
	require.ErrorIs(t, err, repositoryErr)
}

func attemptPolicyForTest(
	t *testing.T,
	repository *authAttemptRepositoryStub,
) *AttemptPolicy {
	t.Helper()
	policy, err := NewAttemptPolicy(AttemptPolicyDependencies{
		Repository:           repository,
		SubjectFingerprinter: &fingerprinterStub{marker: 0x11},
		IPFingerprinter:      &fingerprinterStub{marker: 0x22},
		Now: func() time.Time {
			return time.Date(2026, time.July, 30, 3, 0, 0, 0, time.UTC)
		},
		Config: ports.AuthAttemptPolicy{
			Threshold:    5,
			Window:       15 * time.Minute,
			LockDuration: 30 * time.Minute,
		},
	})
	require.NoError(t, err)
	return policy
}

type fingerprinterStub struct {
	marker byte
	value  string
}

func (fingerprinter *fingerprinterStub) Sum(value string) [32]byte {
	fingerprinter.value = value
	return [32]byte{fingerprinter.marker}
}

type authAttemptRepositoryStub struct {
	checked  []ports.AuthAttemptKey
	failed   []ports.AuthAttemptKey
	reset    ports.AuthAttemptKey
	decision ports.AuthAttemptDecision
	err      error
}

func (repository *authAttemptRepositoryStub) Check(
	_ context.Context,
	keys []ports.AuthAttemptKey,
	_ time.Time,
) (ports.AuthAttemptDecision, error) {
	repository.checked = append([]ports.AuthAttemptKey(nil), keys...)
	return repository.decision, repository.err
}

func (repository *authAttemptRepositoryStub) RecordFailure(
	_ context.Context,
	keys []ports.AuthAttemptKey,
	_ time.Time,
	_ ports.AuthAttemptPolicy,
) (ports.AuthAttemptDecision, error) {
	repository.failed = append([]ports.AuthAttemptKey(nil), keys...)
	return repository.decision, repository.err
}

func (repository *authAttemptRepositoryStub) Reset(
	_ context.Context,
	key ports.AuthAttemptKey,
) error {
	repository.reset = key
	return repository.err
}
