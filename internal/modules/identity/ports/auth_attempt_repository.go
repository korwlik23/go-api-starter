package ports

import (
	"context"
	"time"
)

type AuthAttemptKeyType string

const (
	AuthAttemptKeySubject AuthAttemptKeyType = "subject"
	AuthAttemptKeyIP      AuthAttemptKeyType = "ip"
)

type AuthAttemptKey struct {
	Type AuthAttemptKeyType
	Hash [32]byte
}

type AuthAttemptPolicy struct {
	Threshold    uint64
	Window       time.Duration
	LockDuration time.Duration
}

type AuthAttemptDecision struct {
	LockedUntil *time.Time
}

type AuthAttemptRepository interface {
	Check(
		ctx context.Context,
		keys []AuthAttemptKey,
		now time.Time,
	) (AuthAttemptDecision, error)
	RecordFailure(
		ctx context.Context,
		keys []AuthAttemptKey,
		now time.Time,
		policy AuthAttemptPolicy,
	) (AuthAttemptDecision, error)
	Reset(ctx context.Context, key AuthAttemptKey) error
}
