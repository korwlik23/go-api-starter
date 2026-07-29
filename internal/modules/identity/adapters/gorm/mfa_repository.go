package gorm

import (
	"context"
	"errors"
	"fmt"
	"time"

	identitydomain "github.com/korwlik23/go-api-starter/internal/modules/identity/domain"
	"github.com/korwlik23/go-api-starter/internal/modules/identity/ports"
	gormdb "gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var (
	ErrInvalidMFARepository  = errors.New("invalid MFA repository")
	ErrMFAEnrollmentConflict = errors.New("MFA enrollment conflicts with live credential")
)

type MFARepository struct {
	db *gormdb.DB
}

type mfaTOTPRecord struct {
	UserID                databaseID `gorm:"column:user_id"`
	EnrollmentID          databaseID `gorm:"column:enrollment_id"`
	SecretEnvelopeVersion int16      `gorm:"column:secret_envelope_version"`
	SecretKeyVersion      int64      `gorm:"column:secret_key_version"`
	SecretNonce           []byte     `gorm:"column:secret_nonce"`
	SecretCiphertext      []byte     `gorm:"column:secret_ciphertext"`
	EnrollmentExpiresAt   *time.Time `gorm:"column:enrollment_expires_at"`
	ConfirmedAt           *time.Time `gorm:"column:confirmed_at"`
	LastUsedStep          *int64     `gorm:"column:last_used_step"`
	CreatedAt             time.Time  `gorm:"column:created_at"`
	UpdatedAt             time.Time  `gorm:"column:updated_at"`
}

func (mfaTOTPRecord) TableName() string {
	return "mfa_totp_credentials"
}

func NewMFARepository(db *gormdb.DB) (*MFARepository, error) {
	if db == nil {
		return nil, ErrInvalidMFARepository
	}
	return &MFARepository{db: db}, nil
}

func (repository *MFARepository) SavePending(
	ctx context.Context,
	enrollment identitydomain.MFAEnrollment,
) error {
	if enrollment.State != identitydomain.MFAEnrollmentPending ||
		enrollment.ID == "" ||
		enrollment.UserID == "" ||
		enrollment.KeyVersion < 1 ||
		len(enrollment.Nonce) != 12 ||
		len(enrollment.Ciphertext) < 17 ||
		!enrollment.ExpiresAt.After(enrollment.CreatedAt) {
		return ErrInvalidMFARepository
	}
	userID, err := encodeDatabaseID(repository.db.Dialector.Name(), enrollment.UserID)
	if err != nil {
		return fmt.Errorf("encode MFA user ID: %w", err)
	}
	enrollmentID, err := encodeDatabaseID(
		repository.db.Dialector.Name(),
		enrollment.ID,
	)
	if err != nil {
		return fmt.Errorf("encode MFA enrollment ID: %w", err)
	}

	err = repository.db.WithContext(ctx).Transaction(func(tx *gormdb.DB) error {
		var existing mfaTOTPRecord
		findErr := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("user_id = ?", userID).
			First(&existing).
			Error
		if findErr != nil && !errors.Is(findErr, gormdb.ErrRecordNotFound) {
			return findErr
		}
		if findErr == nil &&
			(existing.ConfirmedAt != nil ||
				existing.EnrollmentExpiresAt == nil ||
				existing.EnrollmentExpiresAt.After(enrollment.CreatedAt)) {
			return ErrMFAEnrollmentConflict
		}

		values := pendingMFAValues(enrollmentID, userID, enrollment)
		if errors.Is(findErr, gormdb.ErrRecordNotFound) {
			return tx.Table("mfa_totp_credentials").Create(values).Error
		}
		return tx.Table("mfa_totp_credentials").
			Where("user_id = ?", userID).
			Updates(values).
			Error
	})
	if err != nil {
		return fmt.Errorf("save pending MFA enrollment: %w", err)
	}
	return nil
}

func pendingMFAValues(
	enrollmentID any,
	userID any,
	enrollment identitydomain.MFAEnrollment,
) map[string]any {
	return map[string]any{
		"user_id":                 userID,
		"enrollment_id":           enrollmentID,
		"secret_envelope_version": int16(1),
		"secret_key_version":      enrollment.KeyVersion,
		"secret_nonce":            append([]byte(nil), enrollment.Nonce...),
		"secret_ciphertext":       append([]byte(nil), enrollment.Ciphertext...),
		"enrollment_expires_at":   enrollment.ExpiresAt,
		"confirmed_at":            nil,
		"last_used_step":          nil,
		"created_at":              enrollment.CreatedAt,
		"updated_at":              enrollment.CreatedAt,
	}
}

func (repository *MFARepository) LoadActive(
	ctx context.Context,
	userID string,
) (identitydomain.MFAEnrollment, error) {
	encodedID, err := encodeDatabaseID(repository.db.Dialector.Name(), userID)
	if err != nil {
		return identitydomain.MFAEnrollment{}, fmt.Errorf("encode MFA user ID: %w", err)
	}
	var record mfaTOTPRecord
	err = repository.db.WithContext(ctx).
		Where("user_id = ? AND confirmed_at IS NOT NULL", encodedID).
		First(&record).
		Error
	if errors.Is(err, gormdb.ErrRecordNotFound) {
		return identitydomain.MFAEnrollment{}, ports.ErrMFAEnrollmentNotFound
	}
	if err != nil {
		return identitydomain.MFAEnrollment{}, fmt.Errorf("load active MFA enrollment: %w", err)
	}
	return identitydomain.MFAEnrollment{
		ID:           record.EnrollmentID.String(),
		UserID:       record.UserID.String(),
		State:        identitydomain.MFAEnrollmentActive,
		KeyVersion:   record.SecretKeyVersion,
		Nonce:        append([]byte(nil), record.SecretNonce...),
		Ciphertext:   append([]byte(nil), record.SecretCiphertext...),
		LastUsedStep: record.LastUsedStep,
		CreatedAt:    record.CreatedAt,
	}, nil
}

func (repository *MFARepository) IsEnabled(
	ctx context.Context,
	userID string,
) (bool, error) {
	encodedID, err := encodeDatabaseID(repository.db.Dialector.Name(), userID)
	if err != nil {
		return false, fmt.Errorf("encode MFA user ID: %w", err)
	}
	var count int64
	err = repository.db.WithContext(ctx).
		Model(&mfaTOTPRecord{}).
		Where("user_id = ? AND confirmed_at IS NOT NULL", encodedID).
		Count(&count).
		Error
	if err != nil {
		return false, fmt.Errorf("read MFA status: %w", err)
	}
	return count == 1, nil
}

func (repository *MFARepository) CompleteTOTP(
	ctx context.Context,
	enrollmentID string,
	step int64,
	pendingHash identitydomain.TokenHash,
	replacement identitydomain.Session,
) error {
	encodedEnrollmentID, err := encodeDatabaseID(
		repository.db.Dialector.Name(),
		enrollmentID,
	)
	if err != nil {
		return fmt.Errorf("encode MFA enrollment ID: %w", err)
	}
	err = repository.db.WithContext(ctx).Transaction(func(tx *gormdb.DB) error {
		result := tx.Model(&mfaTOTPRecord{}).
			Where(
				"enrollment_id = ? AND confirmed_at IS NOT NULL AND "+
					"(last_used_step IS NULL OR last_used_step < ?)",
				encodedEnrollmentID,
				step,
			).
			Updates(map[string]any{
				"last_used_step": step,
				"updated_at":     replacement.CreatedAt,
			})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return ports.ErrMFACompletionConflict
		}

		result = tx.Model(&sessionRecord{}).
			Where(
				"token_hash = ? AND state = ? AND revoked_at IS NULL",
				pendingHash[:],
				string(identitydomain.SessionStateMFAPending),
			).
			Update("revoked_at", replacement.CreatedAt)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return ports.ErrMFACompletionConflict
		}
		return createSession(tx, replacement)
	})
	if err != nil {
		return fmt.Errorf("complete TOTP verification: %w", err)
	}
	return nil
}

var _ ports.MFAStatusReader = (*MFARepository)(nil)
var _ ports.MFAEnrollmentRepository = (*MFARepository)(nil)
var _ ports.MFACompletionRepository = (*MFARepository)(nil)
