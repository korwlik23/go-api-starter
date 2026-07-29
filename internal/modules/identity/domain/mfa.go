package domain

import (
	"errors"
	"time"
)

var ErrInvalidRecoveryCodeHash = errors.New(
	"recovery code hash must contain exactly 32 bytes",
)

type MFAEnrollmentState string

const (
	MFAEnrollmentPending MFAEnrollmentState = "pending"
	MFAEnrollmentActive  MFAEnrollmentState = "active"
)

type MFAEnrollment struct {
	ID           string
	UserID       string
	State        MFAEnrollmentState
	KeyVersion   int64
	Nonce        []byte
	Ciphertext   []byte
	LastUsedStep *int64
	ExpiresAt    time.Time
	CreatedAt    time.Time
}

type MFARecoveryCode struct {
	Hash           [32]byte
	HashVersion    int16
	HashKeyVersion int64
	CreatedAt      time.Time
}
