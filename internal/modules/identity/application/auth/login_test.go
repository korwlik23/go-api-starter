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
	login, err := NewPasswordLogin(PasswordLoginDependencies{
		Users:     users,
		Passwords: passwords,
		Verifier:  verifier,
		DummyHash: "dummy-hash",
	})
	require.NoError(t, err)

	result, err := login.Authenticate(
		context.Background(),
		"  USER@Example.COM ",
		"correct password",
	)
	require.NoError(t, err)
	require.Equal(t, user, result)
	require.Equal(t, "user@example.com", users.email)
	require.Equal(t, user.ID, passwords.userID)
	require.Equal(t, "correct password", verifier.password)
	require.Equal(t, "stored-hash", verifier.hash)
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
			login, err := NewPasswordLogin(PasswordLoginDependencies{
				Users:     testCase.users,
				Passwords: testCase.passwords,
				Verifier:  testCase.verifier,
				DummyHash: "dummy-hash",
			})
			require.NoError(t, err)

			result, err := login.Authenticate(
				context.Background(),
				"user@example.com",
				"candidate",
			)

			require.Equal(t, ErrInvalidCredentials, err)
			require.Empty(t, result)
			require.Equal(t, 1, testCase.verifier.calls)
			require.Equal(t, testCase.expectedHash, testCase.verifier.hash)
		})
	}
}

func TestPasswordLoginFailsClosedOnRepositoryError(t *testing.T) {
	repositoryErr := errors.New("database unavailable")
	login, err := NewPasswordLogin(PasswordLoginDependencies{
		Users:     &userRepositoryStub{err: repositoryErr},
		Passwords: &passwordRepositoryStub{},
		Verifier:  &passwordVerifierStub{},
		DummyHash: "dummy-hash",
	})
	require.NoError(t, err)

	_, err = login.Authenticate(context.Background(), "user@example.com", "candidate")
	require.ErrorIs(t, err, repositoryErr)
	require.NotEqual(t, ErrInvalidCredentials, err)
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
