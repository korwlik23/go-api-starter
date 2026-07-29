package domain

import "time"

type RoleOwnerType string

const (
	RoleOwnerSystem  RoleOwnerType = "system"
	RoleOwnerAccount RoleOwnerType = "account"
)

type Role struct {
	ID        string
	OwnerType RoleOwnerType
	AccountID string
	Name      string
	Version   int64
	CreatedAt time.Time
	UpdatedAt time.Time
}
