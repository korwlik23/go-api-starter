package domain

import (
	"errors"
	"time"
)

var (
	ErrInvalidTokenHash       = errors.New("token hash must contain exactly 32 bytes")
	ErrSessionRevoked         = errors.New("session is revoked")
	ErrSessionIdleExpired     = errors.New("session idle expiry reached")
	ErrSessionAbsoluteExpired = errors.New("session absolute expiry reached")
)

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

func (session Session) ValidateAt(now time.Time) error {
	if session.RevokedAt != nil {
		return ErrSessionRevoked
	}
	if !now.Before(session.AbsoluteExpiresAt) {
		return ErrSessionAbsoluteExpired
	}
	if !now.Before(session.IdleExpiresAt) {
		return ErrSessionIdleExpired
	}
	return nil
}

type SessionState string

const (
	SessionStateAnonymous     SessionState = "anonymous"
	SessionStateMFAPending    SessionState = "mfa_pending"
	SessionStateAuthenticated SessionState = "authenticated"
)
