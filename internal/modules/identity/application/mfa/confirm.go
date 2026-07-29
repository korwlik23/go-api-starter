package mfa

import (
	"context"
	"errors"
	"time"

	identitydomain "github.com/korwlik23/go-api-starter/internal/modules/identity/domain"
	"github.com/korwlik23/go-api-starter/internal/modules/identity/ports"
	platformcrypto "github.com/korwlik23/go-api-starter/internal/platform/crypto"
)

const DefaultRecoveryCodeCount = 10

var ErrInvalidMFAConfirmation = errors.New("invalid MFA confirmation")

type recoveryCodeGenerator interface {
	Generate(userID string) (platformcrypto.RecoveryCode, error)
}

type ConfirmerDependencies struct {
	Repository ports.MFAConfirmationRepository
	Cipher     envelopeDecryptor
	Codes      recoveryCodeGenerator
	Now        func() time.Time
	CodeCount  int
}

type ConfirmCommand struct {
	UserID       string
	EnrollmentID string
	Code         string
}

type ConfirmationResult struct {
	RecoveryCodes []string
}

type Confirmer struct {
	dependencies ConfirmerDependencies
}

func NewConfirmer(
	dependencies ConfirmerDependencies,
) (*Confirmer, error) {
	if dependencies.Repository == nil ||
		dependencies.Cipher == nil ||
		dependencies.Codes == nil ||
		dependencies.Now == nil ||
		dependencies.CodeCount < 1 ||
		dependencies.CodeCount > 20 {
		return nil, ErrInvalidMFAConfirmation
	}
	return &Confirmer{dependencies: dependencies}, nil
}

func (confirmer *Confirmer) Confirm(
	ctx context.Context,
	command ConfirmCommand,
) (ConfirmationResult, error) {
	if command.UserID == "" || command.EnrollmentID == "" || command.Code == "" {
		return ConfirmationResult{}, ErrInvalidMFAConfirmation
	}
	enrollment, err := confirmer.dependencies.Repository.LoadPending(
		ctx,
		command.UserID,
		command.EnrollmentID,
	)
	if err != nil {
		return ConfirmationResult{}, ErrInvalidMFAConfirmation
	}
	now := confirmer.dependencies.Now()
	if enrollment.State != identitydomain.MFAEnrollmentPending ||
		enrollment.UserID != command.UserID ||
		enrollment.ID != command.EnrollmentID ||
		!now.Before(enrollment.ExpiresAt) {
		return ConfirmationResult{}, ErrInvalidMFAConfirmation
	}
	secret, err := confirmer.dependencies.Cipher.Decrypt(
		platformcrypto.Envelope{
			KeyVersion: enrollment.KeyVersion,
			Nonce:      enrollment.Nonce,
			Ciphertext: enrollment.Ciphertext,
		},
		enrollmentAssociatedData(enrollment.UserID, enrollment.ID),
	)
	if err != nil {
		return ConfirmationResult{}, ErrInvalidMFAConfirmation
	}
	defer clear(secret)
	step, valid := platformcrypto.VerifyTOTP(
		string(secret),
		command.Code,
		now,
		-1,
	)
	if !valid {
		return ConfirmationResult{}, ErrInvalidMFAConfirmation
	}

	rawCodes := make([]string, 0, confirmer.dependencies.CodeCount)
	hashes := make([]identitydomain.MFARecoveryCode, 0, confirmer.dependencies.CodeCount)
	for range confirmer.dependencies.CodeCount {
		code, generateErr := confirmer.dependencies.Codes.Generate(command.UserID)
		if generateErr != nil {
			return ConfirmationResult{}, generateErr
		}
		rawCodes = append(rawCodes, code.Raw)
		hashes = append(hashes, identitydomain.MFARecoveryCode{
			Hash:           code.Hash,
			HashVersion:    code.HashVersion,
			HashKeyVersion: code.HashKeyVersion,
			CreatedAt:      now,
		})
	}
	if err := confirmer.dependencies.Repository.Confirm(
		ctx,
		command.UserID,
		command.EnrollmentID,
		step,
		hashes,
		now,
	); err != nil {
		return ConfirmationResult{}, err
	}
	return ConfirmationResult{RecoveryCodes: rawCodes}, nil
}
