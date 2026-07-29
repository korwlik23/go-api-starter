package domain

import "time"

type MFAEnrollmentState string

const (
	MFAEnrollmentPending MFAEnrollmentState = "pending"
	MFAEnrollmentActive  MFAEnrollmentState = "active"
)

type MFAEnrollment struct {
	ID         string
	UserID     string
	State      MFAEnrollmentState
	KeyVersion string
	Nonce      []byte
	Ciphertext []byte
	ExpiresAt  time.Time
	CreatedAt  time.Time
}
