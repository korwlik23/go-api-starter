package mfa

import (
	"context"
	"testing"
	"time"

	identitydomain "github.com/korwlik23/go-api-starter/internal/modules/identity/domain"
	platformcrypto "github.com/korwlik23/go-api-starter/internal/platform/crypto"
	"github.com/stretchr/testify/require"
)

func TestVerifyRecoveryCodeConsumesMatchingCodeAndRotatesSession(t *testing.T) {
	repository := &recoveryCompletionRepositoryStub{
		codes: []identitydomain.MFARecoveryCode{
			{Hash: [32]byte{0x11}, HashVersion: 1, HashKeyVersion: 2},
		},
	}
	verifier, err := NewRecoveryVerifier(RecoveryVerifierDependencies{
		Repository: repository,
		Codes: recoveryCodeMatcherStub{
			matches: true,
		},
		SessionTokens: tokenGeneratorStub{token: platformcrypto.OpaqueToken{
			Raw: "authenticated-session", Hash: [32]byte{0x02},
		}},
		CSRFTokens: tokenGeneratorStub{token: platformcrypto.OpaqueToken{
			Raw: "authenticated-csrf", Hash: [32]byte{0x03},
		}},
		NewID:       func() (string, error) { return "authenticated-id", nil },
		Now:         func() time.Time { return time.Unix(100, 0).UTC() },
		IdleTTL:     30 * time.Minute,
		AbsoluteTTL: 12 * time.Hour,
	})
	require.NoError(t, err)

	result, err := verifier.VerifyRecoveryCode(
		context.Background(),
		VerifyRecoveryCodeCommand{
			PendingSessionHash:   identitydomain.TokenHash{0x01},
			UserID:               "user-id",
			AuthorizationVersion: 7,
			Code:                 "recovery-code",
		},
	)

	require.NoError(t, err)
	require.Equal(t, "authenticated-session", result.SessionToken)
	require.Equal(t, [32]byte{0x11}, repository.consumedHash)
	require.Equal(t, identitydomain.TokenHash{0x01}, repository.pendingHash)
}

type recoveryCodeMatcherStub struct {
	matches bool
}

func (stub recoveryCodeMatcherStub) Matches(
	string,
	string,
	int64,
	[32]byte,
) bool {
	return stub.matches
}

type recoveryCompletionRepositoryStub struct {
	codes        []identitydomain.MFARecoveryCode
	consumedHash [32]byte
	pendingHash  identitydomain.TokenHash
	replacement  identitydomain.Session
}

func (stub *recoveryCompletionRepositoryStub) LoadLiveRecoveryCodes(
	context.Context,
	string,
) ([]identitydomain.MFARecoveryCode, error) {
	return stub.codes, nil
}

func (stub *recoveryCompletionRepositoryStub) CompleteRecoveryCode(
	_ context.Context,
	_ string,
	hash [32]byte,
	pendingHash identitydomain.TokenHash,
	replacement identitydomain.Session,
) error {
	stub.consumedHash = hash
	stub.pendingHash = pendingHash
	stub.replacement = replacement
	return nil
}
