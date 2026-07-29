package domain

import "time"

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
