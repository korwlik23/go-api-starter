package domain

import "time"

type AccountStatus string

const (
	AccountStatusActive   AccountStatus = "active"
	AccountStatusDisabled AccountStatus = "disabled"
)

type Account struct {
	ID        string
	Slug      string
	Status    AccountStatus
	CreatedAt time.Time
	UpdatedAt time.Time
}
