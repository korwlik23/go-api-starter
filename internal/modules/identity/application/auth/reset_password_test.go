package auth

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

func TestResetPasswordCompletesAtomicTokenPasswordAndSessionChange(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.July, 30, 5, 0, 0, 0, time.UTC)
	metadata := ports.PasswordResetTokenMetadata{
		ID:                   "token-id",
		UserID:               "user-id",
		HashVersion:          1,
		HashKeyVersion:       3,
		AuthorizationVersion: 8,
		MFARequired:          false,
	}
	repository := &passwordResetRepositoryStub{metadata: metadata}
	codec := &passwordResetTokenCodecStub{
		tokenID: "token-id",
		hash:    [32]byte{0x42},
	}
	useCase, err := NewResetPassword(passwordResetDependencies(
		now,
		repository,
		codec,
	))
	require.NoError(t, err)

	result, err := useCase.Execute(context.Background(), ResetPasswordCommand{
		RawToken:           "token-id.raw-secret",
		NewPassword:        "correct horse battery staple",
		CurrentSessionHash: identitydomain.TokenHash{0x11},
	})

	require.NoError(t, err)
	require.Equal(t, "new-session-token", result.SessionToken)
	require.Equal(t, "new-csrf-token", result.CSRFToken)
	require.Equal(t, "token-id.raw-secret", codec.raw)
	require.Equal(t, identitydomain.TokenHash{0x42}, repository.command.CandidateHash)
	require.Equal(t, "argon2id-hash", repository.command.NewPasswordHash)
	require.Equal(t, int64(9), *repository.command.Replacement.AuthorizationVersion)
	require.Equal(
		t,
		identitydomain.SessionStateAuthenticated,
		repository.command.Replacement.State,
	)
	require.Equal(t, identitydomain.TokenHash{0x11}, repository.command.CurrentSessionHash)
	require.Equal(t, now, repository.command.Now)
}

func TestResetPasswordCreatesMFAPendingReplacementWhenRequired(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.July, 30, 5, 0, 0, 0, time.UTC)
	repository := &passwordResetRepositoryStub{
		metadata: ports.PasswordResetTokenMetadata{
			ID:                   "token-id",
			UserID:               "user-id",
			HashVersion:          1,
			HashKeyVersion:       3,
			AuthorizationVersion: 2,
			MFARequired:          true,
		},
	}
	useCase, err := NewResetPassword(passwordResetDependencies(
		now,
		repository,
		&passwordResetTokenCodecStub{
			tokenID: "token-id",
			hash:    [32]byte{0x42},
		},
	))
	require.NoError(t, err)

	_, err = useCase.Execute(context.Background(), ResetPasswordCommand{
		RawToken:    "token-id.raw-secret",
		NewPassword: "a secure replacement password",
	})

	require.NoError(t, err)
	require.Equal(
		t,
		identitydomain.SessionStateMFAPending,
		repository.command.Replacement.State,
	)
}

func TestResetPasswordRejectsMalformedTokenWithoutLookup(t *testing.T) {
	t.Parallel()

	repository := &passwordResetRepositoryStub{}
	dependencies := passwordResetDependencies(
		time.Now(),
		repository,
		&passwordResetTokenCodecStub{valid: false},
	)
	useCase, err := NewResetPassword(dependencies)
	require.NoError(t, err)

	result, err := useCase.Execute(context.Background(), ResetPasswordCommand{
		RawToken:    "malformed",
		NewPassword: "a secure replacement password",
	})

	require.ErrorIs(t, err, ErrRecoveryTokenInvalid)
	require.Empty(t, result)
	require.Zero(t, repository.metadataCalls)
	require.NotContains(t, err.Error(), "malformed")
}

func TestResetPasswordReturnsNoTokensWhenTransactionFails(t *testing.T) {
	t.Parallel()

	transactionFailure := errors.New("reset conflict")
	repository := &passwordResetRepositoryStub{
		metadata: ports.PasswordResetTokenMetadata{
			ID:                   "token-id",
			UserID:               "user-id",
			HashVersion:          1,
			HashKeyVersion:       3,
			AuthorizationVersion: 1,
		},
		completeErr: transactionFailure,
	}
	useCase, err := NewResetPassword(passwordResetDependencies(
		time.Now(),
		repository,
		&passwordResetTokenCodecStub{
			tokenID: "token-id",
			hash:    [32]byte{0x42},
		},
	))
	require.NoError(t, err)

	result, err := useCase.Execute(context.Background(), ResetPasswordCommand{
		RawToken:    "token-id.raw-secret",
		NewPassword: "a secure replacement password",
	})

	require.ErrorIs(t, err, transactionFailure)
	require.Empty(t, result)
}

func passwordResetDependencies(
	now time.Time,
	repository *passwordResetRepositoryStub,
	codec *passwordResetTokenCodecStub,
) ResetPasswordDependencies {
	return ResetPasswordDependencies{
		Repository: repository,
		Tokens:     codec,
		Passwords:  passwordResetHasherStub{},
		SessionTokens: &sequenceTokenGenerator{
			token: platformcrypto.OpaqueToken{
				Raw:  "new-session-token",
				Hash: [32]byte{0x31},
			},
		},
		CSRFTokens: &sequenceTokenGenerator{
			token: platformcrypto.OpaqueToken{
				Raw:  "new-csrf-token",
				Hash: [32]byte{0x32},
			},
		},
		NewID:          func() (string, error) { return "new-session-id", nil },
		Now:            func() time.Time { return now },
		IdleTTL:        30 * time.Minute,
		AbsoluteTTL:    12 * time.Hour,
		MFAIdleTTL:     5 * time.Minute,
		MFAAbsoluteTTL: 10 * time.Minute,
	}
}

type passwordResetTokenCodecStub struct {
	tokenID string
	hash    [32]byte
	raw     string
	valid   bool
}

func (stub *passwordResetTokenCodecStub) Parse(raw string) (string, bool) {
	if stub.valid == false && stub.tokenID == "" {
		return "", false
	}
	return stub.tokenID, true
}

func (stub *passwordResetTokenCodecStub) Hash(
	_ string,
	_ string,
	_ string,
	raw string,
	_ int64,
) [32]byte {
	stub.raw = raw
	return stub.hash
}

type passwordResetHasherStub struct{}

func (passwordResetHasherStub) Hash(string) (string, error) {
	return "argon2id-hash", nil
}

type sequenceTokenGenerator struct {
	token platformcrypto.OpaqueToken
}

func (generator *sequenceTokenGenerator) Generate() (platformcrypto.OpaqueToken, error) {
	return generator.token, nil
}

type passwordResetRepositoryStub struct {
	metadata      ports.PasswordResetTokenMetadata
	command       ports.CompletePasswordResetCommand
	metadataErr   error
	completeErr   error
	metadataCalls int
}

func (stub *passwordResetRepositoryStub) FindPasswordResetToken(
	_ context.Context,
	_ string,
) (ports.PasswordResetTokenMetadata, error) {
	stub.metadataCalls++
	return stub.metadata, stub.metadataErr
}

func (stub *passwordResetRepositoryStub) CompletePasswordReset(
	_ context.Context,
	command ports.CompletePasswordResetCommand,
) error {
	stub.command = command
	return stub.completeErr
}
