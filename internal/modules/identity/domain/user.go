package domain

import (
	"strings"
	"time"
)

type UserStatus string

const (
	UserStatusActive   UserStatus = "active"
	UserStatusDisabled UserStatus = "disabled"
)

type User struct {
	ID                   string
	Email                string
	Status               UserStatus
	AuthorizationVersion int64
	CreatedAt            time.Time
	UpdatedAt            time.Time
}

func NormalizeEmail(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}
