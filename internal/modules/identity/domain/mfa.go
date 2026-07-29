package domain

type MFAEnrollmentState string

const (
	MFAEnrollmentPending MFAEnrollmentState = "pending"
	MFAEnrollmentActive  MFAEnrollmentState = "active"
)
