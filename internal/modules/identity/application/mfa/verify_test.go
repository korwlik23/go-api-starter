package mfa

import (
	"context"
	"errors"
	"testing"
	"time"

	identitydomain "github.com/korwlik23/go-api-starter/internal/modules/identity/domain"
	"github.com/korwlik23/go-api-starter/internal/modules/identity/ports"
	platformcrypto "github.com/korwlik23/go-api-starter/internal/platform/crypto"
	"github.com/stretchr/testify/require"
)

func TestVerifyTOTPConsumesNewStepAndRotatesAuthenticatedSession(t *testing.T) {
	now := time.Unix(59, 0).UTC()
	repository := &mfaCompletionRepositoryStub{
		active: identitydomain.MFAEnrollment{
			ID:           "enrollment-id",
			UserID:       "user-id",
			State:        identitydomain.MFAEnrollmentActive,
			KeyVersion:   1,
			Nonce:        make([]byte, 12),
			Ciphertext:   []byte("encrypted"),
			LastUsedStep: int64PointerValue(0),
		},
	}
	verifier, err := NewVerifier(VerifierDependencies{
		Repository: repository,
		Cipher: &envelopeDecryptorStub{
			plaintext: []byte("GEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQ"),
		},
		SessionTokens: tokenGeneratorStub{token: platformcrypto.OpaqueToken{
			Raw: "authenticated-session", Hash: [32]byte{0x02},
		}},
		CSRFTokens: tokenGeneratorStub{token: platformcrypto.OpaqueToken{
			Raw: "authenticated-csrf", Hash: [32]byte{0x03},
		}},
		NewID:       func() (string, error) { return "authenticated-id", nil },
		Now:         func() time.Time { return now },
		IdleTTL:     30 * time.Minute,
		AbsoluteTTL: 12 * time.Hour,
	})
	require.NoError(t, err)

	result, err := verifier.VerifyTOTP(context.Background(), VerifyTOTPCommand{
		PendingSessionHash:   identitydomain.TokenHash{0x01},
		UserID:               "user-id",
		AuthorizationVersion: 7,
		Code:                 "287082",
	})
	require.NoError(t, err)
	require.Equal(t, "authenticated-session", result.SessionToken)
	require.Equal(t, "authenticated-csrf", result.CSRFToken)
	require.Equal(t, int64(1), repository.step)
	require.Equal(t, identitydomain.SessionStateAuthenticated, repository.replacement.State)
	require.Equal(t, identitydomain.TokenHash{0x01}, repository.pendingHash)
}

func TestVerifyTOTPRejectsReplayAndReturnsNoTokens(t *testing.T) {
	repository := &mfaCompletionRepositoryStub{
		active: identitydomain.MFAEnrollment{
			ID:           "enrollment-id",
			UserID:       "user-id",
			State:        identitydomain.MFAEnrollmentActive,
			LastUsedStep: int64PointerValue(1),
		},
	}
	verifier := mfaVerifierForTest(t, repository)

	result, err := verifier.VerifyTOTP(context.Background(), VerifyTOTPCommand{
		PendingSessionHash:   identitydomain.TokenHash{0x01},
		UserID:               "user-id",
		AuthorizationVersion: 7,
		Code:                 "287082",
	})
	require.Equal(t, ErrInvalidMFAVerification, err)
	require.Empty(t, result)
	require.Equal(t, 0, repository.completeCalls)
}

func TestVerifyTOTPReturnsNoTokensWhenConcurrentCompletionLoses(t *testing.T) {
	repository := &mfaCompletionRepositoryStub{
		active: identitydomain.MFAEnrollment{
			ID:     "enrollment-id",
			UserID: "user-id",
			State:  identitydomain.MFAEnrollmentActive,
		},
		completeErr: ports.ErrMFACompletionConflict,
	}
	verifier := mfaVerifierForTest(t, repository)

	result, err := verifier.VerifyTOTP(context.Background(), VerifyTOTPCommand{
		PendingSessionHash:   identitydomain.TokenHash{0x01},
		UserID:               "user-id",
		AuthorizationVersion: 7,
		Code:                 "287082",
	})
	require.ErrorIs(t, err, ports.ErrMFACompletionConflict)
	require.Empty(t, result)
}

func mfaVerifierForTest(
	t *testing.T,
	repository *mfaCompletionRepositoryStub,
) *Verifier {
	t.Helper()
	verifier, err := NewVerifier(VerifierDependencies{
		Repository: repository,
		Cipher: &envelopeDecryptorStub{
			plaintext: []byte("GEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQ"),
		},
		SessionTokens: tokenGeneratorStub{token: platformcrypto.OpaqueToken{
			Raw: "authenticated-session", Hash: [32]byte{0x02},
		}},
		CSRFTokens: tokenGeneratorStub{token: platformcrypto.OpaqueToken{
			Raw: "authenticated-csrf", Hash: [32]byte{0x03},
		}},
		NewID:       func() (string, error) { return "authenticated-id", nil },
		Now:         func() time.Time { return time.Unix(59, 0).UTC() },
		IdleTTL:     30 * time.Minute,
		AbsoluteTTL: 12 * time.Hour,
	})
	require.NoError(t, err)
	return verifier
}

type envelopeDecryptorStub struct {
	plaintext []byte
	err       error
}

func (cipher *envelopeDecryptorStub) Decrypt(
	platformcrypto.Envelope,
	[]byte,
) ([]byte, error) {
	return append([]byte(nil), cipher.plaintext...), cipher.err
}

type mfaCompletionRepositoryStub struct {
	active        identitydomain.MFAEnrollment
	loadErr       error
	completeErr   error
	step          int64
	pendingHash   identitydomain.TokenHash
	replacement   identitydomain.Session
	completeCalls int
}

func (repository *mfaCompletionRepositoryStub) LoadActive(
	context.Context,
	string,
) (identitydomain.MFAEnrollment, error) {
	return repository.active, repository.loadErr
}

func (repository *mfaCompletionRepositoryStub) CompleteTOTP(
	_ context.Context,
	enrollmentID string,
	step int64,
	pendingHash identitydomain.TokenHash,
	replacement identitydomain.Session,
) error {
	if enrollmentID != repository.active.ID {
		return errors.New("unexpected enrollment")
	}
	repository.completeCalls++
	repository.step = step
	repository.pendingHash = pendingHash
	repository.replacement = replacement
	return repository.completeErr
}

type tokenGeneratorStub struct {
	token platformcrypto.OpaqueToken
	err   error
}

func (generator tokenGeneratorStub) Generate() (platformcrypto.OpaqueToken, error) {
	return generator.token, generator.err
}

func int64PointerValue(value int64) *int64 {
	return &value
}
