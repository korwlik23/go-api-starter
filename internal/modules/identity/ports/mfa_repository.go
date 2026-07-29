package ports

import (
	"context"
	"errors"
	"time"

	"github.com/korwlik23/go-api-starter/internal/modules/identity/domain"
)

var (
	ErrMFACompletionConflict = errors.New("MFA completion conflict")
	ErrMFAEnrollmentNotFound = errors.New("MFA enrollment not found")
)

type MFAStatusReader interface {
	IsEnabled(ctx context.Context, userID string) (bool, error)
}

type MFAEnrollmentRepository interface {
	SavePending(ctx context.Context, enrollment domain.MFAEnrollment) error
}

type MFAConfirmationRepository interface {
	LoadPending(
		ctx context.Context,
		userID string,
		enrollmentID string,
	) (domain.MFAEnrollment, error)
	Confirm(
		ctx context.Context,
		userID string,
		enrollmentID string,
		step int64,
		codes []domain.MFARecoveryCode,
		confirmedAt time.Time,
	) error
}

type MFACompletionRepository interface {
	LoadActive(ctx context.Context, userID string) (domain.MFAEnrollment, error)
	CompleteTOTP(
		ctx context.Context,
		enrollmentID string,
		step int64,
		pendingHash domain.TokenHash,
		replacement domain.Session,
	) error
}

type MFARecoveryCompletionRepository interface {
	LoadLiveRecoveryCodes(
		ctx context.Context,
		userID string,
	) ([]domain.MFARecoveryCode, error)
	CompleteRecoveryCode(
		ctx context.Context,
		userID string,
		codeHash [32]byte,
		pendingHash domain.TokenHash,
		replacement domain.Session,
	) error
}
