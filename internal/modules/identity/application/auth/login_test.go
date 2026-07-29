package auth

import (
	"context"
	"errors"
	"testing"

	identitydomain "github.com/korwlik23/go-api-starter/internal/modules/identity/domain"
	"github.com/korwlik23/go-api-starter/internal/modules/identity/ports"
	"github.com/stretchr/testify/require"
)

func TestPasswordLoginAuthenticatesActiveUser(t *testing.T) {
	user := identitydomain.User{
		ID:     "018f1f64-7b2a-7c10-8bf2-3c40d5f67586",
		Email:  "user@example.com",
		Status: identitydomain.UserStatusActive,
	}
	users := &userRepositoryStub{user: user}
	passwords := &passwordRepositoryStub{hash: "stored-hash"}
	verifier := &passwordVerifierStub{matches: true}
	attempts := &loginAttemptPolicyStub{}
	login, err := NewPasswordLogin(PasswordLoginDependencies{
		Users:     users,
		Passwords: passwords,
		Verifier:  verifier,
		Attempts:  attempts,
		DummyHash: "dummy-hash",
	})
	require.NoError(t, err)

	result, err := login.Authenticate(
		context.Background(),
		"  USER@Example.COM ",
		"correct password",
		"192.0.2.1",
	)
	require.NoError(t, err)
	require.Equal(t, user, result)
	require.Equal(t, "user@example.com", users.email)
	require.Equal(t, user.ID, passwords.userID)
	require.Equal(t, "correct password", verifier.password)
	require.Equal(t, "stored-hash", verifier.hash)
	require.Equal(t, 1, attempts.checkCalls)
	require.Equal(t, 1, attempts.successCalls)
	require.Equal(t, 0, attempts.failureCalls)
}

func TestPasswordLoginUsesGenericFailureAndDummyVerification(t *testing.T) {
	testCases := []struct {
		name         string
		users        *userRepositoryStub
		passwords    *passwordRepositoryStub
		verifier     *passwordVerifierStub
		expectedHash string
	}{
		{
			name:         "unknown user",
			users:        &userRepositoryStub{err: ports.ErrUserNotFound},
			passwords:    &passwordRepositoryStub{},
			verifier:     &passwordVerifierStub{},
			expectedHash: "dummy-hash",
		},
		{
			name: "disabled user",
			users: &userRepositoryStub{user: identitydomain.User{
				ID:     "disabled-id",
				Status: identitydomain.UserStatusDisabled,
			}},
			passwords:    &passwordRepositoryStub{},
			verifier:     &passwordVerifierStub{},
			expectedHash: "dummy-hash",
		},
		{
			name: "wrong password",
			users: &userRepositoryStub{user: identitydomain.User{
				ID:     "active-id",
				Status: identitydomain.UserStatusActive,
			}},
			passwords:    &passwordRepositoryStub{hash: "stored-hash"},
			verifier:     &passwordVerifierStub{matches: false},
			expectedHash: "stored-hash",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			attempts := &loginAttemptPolicyStub{}
			login, err := NewPasswordLogin(PasswordLoginDependencies{
				Users:     testCase.users,
				Passwords: testCase.passwords,
				Verifier:  testCase.verifier,
				Attempts:  attempts,
				DummyHash: "dummy-hash",
			})
			require.NoError(t, err)

			result, err := login.Authenticate(
				context.Background(),
				"user@example.com",
				"candidate",
				"192.0.2.1",
			)

			require.Equal(t, ErrInvalidCredentials, err)
			require.Empty(t, result)
			require.Equal(t, 1, testCase.verifier.calls)
			require.Equal(t, testCase.expectedHash, testCase.verifier.hash)
			require.Equal(t, 1, attempts.failureCalls)
			require.Equal(t, 0, attempts.successCalls)
		})
	}
}

func TestPasswordLoginFailsClosedOnRepositoryError(t *testing.T) {
	repositoryErr := errors.New("database unavailable")
	login, err := NewPasswordLogin(PasswordLoginDependencies{
		Users:     &userRepositoryStub{err: repositoryErr},
		Passwords: &passwordRepositoryStub{},
		Verifier:  &passwordVerifierStub{},
		Attempts:  &loginAttemptPolicyStub{},
		DummyHash: "dummy-hash",
	})
	require.NoError(t, err)

	_, err = login.Authenticate(
		context.Background(),
		"user@example.com",
		"candidate",
		"192.0.2.1",
	)
	require.ErrorIs(t, err, repositoryErr)
	require.NotEqual(t, ErrInvalidCredentials, err)
}

func TestPasswordLoginHidesTemporaryLockAndPerformsDummyVerification(t *testing.T) {
	attempts := &loginAttemptPolicyStub{checkErr: ErrAuthenticationTemporarilyLocked}
	verifier := &passwordVerifierStub{}
	login, err := NewPasswordLogin(PasswordLoginDependencies{
		Users:     &userRepositoryStub{},
		Passwords: &passwordRepositoryStub{},
		Verifier:  verifier,
		Attempts:  attempts,
		DummyHash: "dummy-hash",
	})
	require.NoError(t, err)

	_, err = login.Authenticate(
		context.Background(),
		"user@example.com",
		"candidate",
		"192.0.2.1",
	)

	require.Equal(t, ErrInvalidCredentials, err)
	require.Equal(t, 1, verifier.calls)
	require.Equal(t, "dummy-hash", verifier.hash)
	require.Equal(t, 0, attempts.failureCalls)
}

type userRepositoryStub struct {
	user  identitydomain.User
	err   error
	email string
}

func (repository *userRepositoryStub) FindByNormalizedEmail(
	_ context.Context,
	email string,
) (identitydomain.User, error) {
	repository.email = email
	return repository.user, repository.err
}

type passwordRepositoryStub struct {
	hash   string
	err    error
	userID string
}

func (repository *passwordRepositoryStub) FindPasswordHash(
	_ context.Context,
	userID string,
) (string, error) {
	repository.userID = userID
	return repository.hash, repository.err
}

type passwordVerifierStub struct {
	matches  bool
	err      error
	password string
	hash     string
	calls    int
}

func (verifier *passwordVerifierStub) Verify(password, hash string) (bool, error) {
	verifier.calls++
	verifier.password = password
	verifier.hash = hash
	return verifier.matches, verifier.err
}

type loginAttemptPolicyStub struct {
	keys         []ports.AuthAttemptKey
	checkErr     error
	failureErr   error
	successErr   error
	checkCalls   int
	failureCalls int
	successCalls int
}

func (policy *loginAttemptPolicyStub) Check(
	context.Context,
	string,
	string,
) ([]ports.AuthAttemptKey, error) {
	policy.checkCalls++
	if policy.keys == nil {
		policy.keys = []ports.AuthAttemptKey{
			{Type: ports.AuthAttemptKeySubject},
			{Type: ports.AuthAttemptKeyIP},
		}
	}
	return policy.keys, policy.checkErr
}

func (policy *loginAttemptPolicyStub) RecordFailure(
	context.Context,
	[]ports.AuthAttemptKey,
) error {
	policy.failureCalls++
	return policy.failureErr
}

func (policy *loginAttemptPolicyStub) RecordSuccess(
	context.Context,
	[]ports.AuthAttemptKey,
) error {
	policy.successCalls++
	return policy.successErr
}
