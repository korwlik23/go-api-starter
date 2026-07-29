package mfa

import (
	"context"
	"time"

	identitydomain "github.com/korwlik23/go-api-starter/internal/modules/identity/domain"
	"github.com/korwlik23/go-api-starter/internal/modules/identity/ports"
	platformcrypto "github.com/korwlik23/go-api-starter/internal/platform/crypto"
)

type recoveryCodeMatcher interface {
	Matches(
		userID string,
		raw string,
		keyVersion int64,
		expected [32]byte,
	) bool
}

type RecoveryVerifierDependencies struct {
	Repository    ports.MFARecoveryCompletionRepository
	Codes         recoveryCodeMatcher
	SessionTokens mfaTokenGenerator
	CSRFTokens    mfaTokenGenerator
	NewID         func() (string, error)
	Now           func() time.Time
	IdleTTL       time.Duration
	AbsoluteTTL   time.Duration
}

type VerifyRecoveryCodeCommand struct {
	PendingSessionHash   identitydomain.TokenHash
	UserID               string
	AuthorizationVersion int64
	Code                 string
}

type RecoveryVerifier struct {
	dependencies RecoveryVerifierDependencies
}

func NewRecoveryVerifier(
	dependencies RecoveryVerifierDependencies,
) (*RecoveryVerifier, error) {
	if dependencies.Repository == nil ||
		dependencies.Codes == nil ||
		dependencies.SessionTokens == nil ||
		dependencies.CSRFTokens == nil ||
		dependencies.NewID == nil ||
		dependencies.Now == nil ||
		dependencies.IdleTTL <= 0 ||
		dependencies.AbsoluteTTL <= 0 ||
		dependencies.IdleTTL > dependencies.AbsoluteTTL {
		return nil, ErrInvalidMFAVerification
	}
	return &RecoveryVerifier{dependencies: dependencies}, nil
}

func (verifier *RecoveryVerifier) VerifyRecoveryCode(
	ctx context.Context,
	command VerifyRecoveryCodeCommand,
) (VerificationResult, error) {
	if command.UserID == "" ||
		command.AuthorizationVersion < 1 ||
		command.Code == "" {
		return VerificationResult{}, ErrInvalidMFAVerification
	}
	codes, err := verifier.dependencies.Repository.LoadLiveRecoveryCodes(
		ctx,
		command.UserID,
	)
	if err != nil {
		return VerificationResult{}, err
	}
	var matched *identitydomain.MFARecoveryCode
	for index := range codes {
		if verifier.dependencies.Codes.Matches(
			command.UserID,
			command.Code,
			codes[index].HashKeyVersion,
			codes[index].Hash,
		) {
			matched = &codes[index]
		}
	}
	if matched == nil {
		return VerificationResult{}, ErrInvalidMFAVerification
	}

	sessionToken, err := verifier.dependencies.SessionTokens.Generate()
	if err != nil {
		return VerificationResult{}, err
	}
	csrfToken, err := verifier.dependencies.CSRFTokens.Generate()
	if err != nil {
		return VerificationResult{}, err
	}
	sessionID, err := verifier.dependencies.NewID()
	if err != nil {
		return VerificationResult{}, err
	}
	now := verifier.dependencies.Now()
	replacement := identitydomain.Session{
		ID:                   sessionID,
		TokenHash:            identitydomain.TokenHash(sessionToken.Hash),
		CSRFTokenHash:        identitydomain.TokenHash(csrfToken.Hash),
		State:                identitydomain.SessionStateAuthenticated,
		UserID:               mfaStringPointer(command.UserID),
		AuthorizationVersion: mfaInt64Pointer(command.AuthorizationVersion),
		IdleExpiresAt:        now.Add(verifier.dependencies.IdleTTL),
		AbsoluteExpiresAt:    now.Add(verifier.dependencies.AbsoluteTTL),
		CreatedAt:            now,
	}
	if err := verifier.dependencies.Repository.CompleteRecoveryCode(
		ctx,
		command.UserID,
		matched.Hash,
		command.PendingSessionHash,
		replacement,
	); err != nil {
		return VerificationResult{}, err
	}
	return VerificationResult{
		SessionToken: sessionToken.Raw,
		CSRFToken:    csrfToken.Raw,
	}, nil
}

var _ recoveryCodeMatcher = (*platformcrypto.RecoveryCodeCodec)(nil)
