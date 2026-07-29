package domain

import "time"

type MembershipStatus string

const (
	MembershipStatusActive   MembershipStatus = "active"
	MembershipStatusDisabled MembershipStatus = "disabled"
)

type Membership struct {
	ID                   string
	AccountID            string
	UserID               string
	Status               MembershipStatus
	AuthorizationVersion int64
	CreatedAt            time.Time
	UpdatedAt            time.Time
}
