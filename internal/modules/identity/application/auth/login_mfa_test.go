package auth

import (
	"context"
	"testing"

	sessionapp "github.com/korwlik23/go-api-starter/internal/modules/identity/application/session"
	identitydomain "github.com/korwlik23/go-api-starter/internal/modules/identity/domain"
	"github.com/stretchr/testify/require"
)

func TestLoginFlowRotatesToMFAPendingWhenEnabled(t *testing.T) {
	authenticator := &passwordAuthenticatorStub{user: identitydomain.User{
		ID:                   "user-id",
		Status:               identitydomain.UserStatusActive,
		AuthorizationVersion: 7,
	}}
	mfa := &mfaStatusReaderStub{enabled: true}
	rotator := &loginSessionRotatorStub{
		pendingResult: sessionapp.RotationResult{
			SessionID:    "challenge-id",
			SessionToken: "pending-session",
			CSRFToken:    "pending-csrf",
		},
	}
	flow, err := NewLoginFlow(authenticator, mfa, rotator)
	require.NoError(t, err)

	result, err := flow.Execute(context.Background(), LoginCommand{
		Email:              "user@example.com",
		Password:           "password",
		ClientIP:           "192.0.2.1",
		CurrentSessionHash: identitydomain.TokenHash{0x01},
	})
	require.NoError(t, err)
	require.Equal(t, LoginStatusMFAPending, result.Status)
	require.Equal(t, "challenge-id", result.ChallengeID)
	require.Equal(t, "pending-session", result.SessionToken)
	require.Equal(t, "pending-csrf", result.CSRFToken)
	require.Equal(t, 1, rotator.pendingCalls)
	require.Equal(t, 0, rotator.authenticatedCalls)
}

func TestLoginFlowRotatesDirectlyToAuthenticatedWithoutMFA(t *testing.T) {
	authenticator := &passwordAuthenticatorStub{user: identitydomain.User{
		ID:                   "user-id",
		Status:               identitydomain.UserStatusActive,
		AuthorizationVersion: 7,
	}}
	mfa := &mfaStatusReaderStub{}
	rotator := &loginSessionRotatorStub{
		authenticatedResult: sessionapp.RotationResult{
			SessionID:    "authenticated-id",
			SessionToken: "authenticated-session",
			CSRFToken:    "authenticated-csrf",
		},
	}
	flow, err := NewLoginFlow(authenticator, mfa, rotator)
	require.NoError(t, err)

	result, err := flow.Execute(context.Background(), LoginCommand{
		Email:              "user@example.com",
		Password:           "password",
		ClientIP:           "192.0.2.1",
		CurrentSessionHash: identitydomain.TokenHash{0x01},
	})
	require.NoError(t, err)
	require.Equal(t, LoginStatusAuthenticated, result.Status)
	require.Empty(t, result.ChallengeID)
	require.Equal(t, 0, rotator.pendingCalls)
	require.Equal(t, 1, rotator.authenticatedCalls)
}

type passwordAuthenticatorStub struct {
	user identitydomain.User
	err  error
}

func (authenticator *passwordAuthenticatorStub) Authenticate(
	context.Context,
	string,
	string,
	string,
) (identitydomain.User, error) {
	return authenticator.user, authenticator.err
}

type mfaStatusReaderStub struct {
	enabled bool
	err     error
}

func (reader *mfaStatusReaderStub) IsEnabled(
	context.Context,
	string,
) (bool, error) {
	return reader.enabled, reader.err
}

type loginSessionRotatorStub struct {
	authenticatedResult sessionapp.RotationResult
	pendingResult       sessionapp.RotationResult
	err                 error
	authenticatedCalls  int
	pendingCalls        int
}

func (rotator *loginSessionRotatorStub) Authenticated(
	context.Context,
	identitydomain.TokenHash,
	string,
	int64,
) (sessionapp.RotationResult, error) {
	rotator.authenticatedCalls++
	return rotator.authenticatedResult, rotator.err
}

func (rotator *loginSessionRotatorStub) MFAPending(
	context.Context,
	identitydomain.TokenHash,
	string,
	int64,
) (sessionapp.RotationResult, error) {
	rotator.pendingCalls++
	return rotator.pendingResult, rotator.err
}
