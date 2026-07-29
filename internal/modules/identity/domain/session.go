package domain

import (
	"errors"
	"time"
)

var ErrInvalidTokenHash = errors.New("token hash must contain exactly 32 bytes")

type TokenHash [32]byte

func NewTokenHash(value []byte) (TokenHash, error) {
	if len(value) != len(TokenHash{}) {
		return TokenHash{}, ErrInvalidTokenHash
	}

	var hash TokenHash
	copy(hash[:], value)
	return hash, nil
}

type Session struct {
	ID                   string
	TokenHash            TokenHash
	CSRFTokenHash        TokenHash
	State                SessionState
	UserID               *string
	AuthorizationVersion *int64
	IdleExpiresAt        time.Time
	AbsoluteExpiresAt    time.Time
	RevokedAt            *time.Time
	CreatedAt            time.Time
}

type SessionState string

const (
	SessionStateAnonymous     SessionState = "anonymous"
	SessionStateMFAPending    SessionState = "mfa_pending"
	SessionStateAuthenticated SessionState = "authenticated"
)
