package domain

import "strings"

type UserStatus string

const (
	UserStatusActive   UserStatus = "active"
	UserStatusDisabled UserStatus = "disabled"
)

type User struct {
	ID     string
	Email  string
	Status UserStatus
}

func NormalizeEmail(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}
