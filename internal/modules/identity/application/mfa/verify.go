package mfa

import (
	"context"
	"errors"
	"time"

	identitydomain "github.com/korwlik23/go-api-starter/internal/modules/identity/domain"
	"github.com/korwlik23/go-api-starter/internal/modules/identity/ports"
	platformcrypto "github.com/korwlik23/go-api-starter/internal/platform/crypto"
)

var ErrInvalidMFAVerification = errors.New("invalid MFA verification")

type envelopeDecryptor interface {
	Decrypt(
		envelope platformcrypto.Envelope,
		associatedData []byte,
	) ([]byte, error)
}

type mfaTokenGenerator interface {
	Generate() (platformcrypto.OpaqueToken, error)
}

type VerifierDependencies struct {
	Repository    ports.MFACompletionRepository
	Cipher        envelopeDecryptor
	SessionTokens mfaTokenGenerator
	CSRFTokens    mfaTokenGenerator
	NewID         func() (string, error)
	Now           func() time.Time
	IdleTTL       time.Duration
	AbsoluteTTL   time.Duration
}

type VerifyTOTPCommand struct {
	PendingSessionHash   identitydomain.TokenHash
	UserID               string
	AuthorizationVersion int64
	Code                 string
}

type VerificationResult struct {
	SessionToken string
	CSRFToken    string
}

type Verifier struct {
	dependencies VerifierDependencies
}

func NewVerifier(dependencies VerifierDependencies) (*Verifier, error) {
	if dependencies.Repository == nil ||
		dependencies.Cipher == nil ||
		dependencies.SessionTokens == nil ||
		dependencies.CSRFTokens == nil ||
		dependencies.NewID == nil ||
		dependencies.Now == nil ||
		dependencies.IdleTTL <= 0 ||
		dependencies.AbsoluteTTL <= 0 ||
		dependencies.IdleTTL > dependencies.AbsoluteTTL {
		return nil, ErrInvalidMFAVerification
	}
	return &Verifier{dependencies: dependencies}, nil
}

func (verifier *Verifier) VerifyTOTP(
	ctx context.Context,
	command VerifyTOTPCommand,
) (VerificationResult, error) {
	if command.UserID == "" || command.AuthorizationVersion < 1 {
		return VerificationResult{}, ErrInvalidMFAVerification
	}
	enrollment, err := verifier.dependencies.Repository.LoadActive(ctx, command.UserID)
	if err != nil {
		return VerificationResult{}, err
	}
	if enrollment.State != identitydomain.MFAEnrollmentActive ||
		enrollment.UserID != command.UserID {
		return VerificationResult{}, ErrInvalidMFAVerification
	}

	secret, err := verifier.dependencies.Cipher.Decrypt(
		platformcrypto.Envelope{
			KeyVersion: enrollment.KeyVersion,
			Nonce:      enrollment.Nonce,
			Ciphertext: enrollment.Ciphertext,
		},
		enrollmentAssociatedData(enrollment.UserID, enrollment.ID),
	)
	if err != nil {
		return VerificationResult{}, ErrInvalidMFAVerification
	}
	defer clear(secret)

	lastUsedStep := int64(-1)
	if enrollment.LastUsedStep != nil {
		lastUsedStep = *enrollment.LastUsedStep
	}
	now := verifier.dependencies.Now()
	step, valid := platformcrypto.VerifyTOTP(string(secret), command.Code, now, lastUsedStep)
	if !valid {
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
	if err := verifier.dependencies.Repository.CompleteTOTP(
		ctx,
		enrollment.ID,
		step,
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

func mfaStringPointer(value string) *string {
	return &value
}

func mfaInt64Pointer(value int64) *int64 {
	return &value
}
