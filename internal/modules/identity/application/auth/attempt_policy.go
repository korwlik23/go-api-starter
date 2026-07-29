package auth

import (
	"context"
	"errors"
	"net/netip"
	"time"

	identitydomain "github.com/korwlik23/go-api-starter/internal/modules/identity/domain"
	"github.com/korwlik23/go-api-starter/internal/modules/identity/ports"
)

var (
	ErrAuthenticationTemporarilyLocked = errors.New("authentication temporarily locked")
	ErrInvalidAttemptPolicy            = errors.New("invalid authentication attempt policy")
	ErrInvalidClientIP                 = errors.New("invalid client IP")
)

type attemptFingerprinter interface {
	Sum(value string) [32]byte
}

type AttemptPolicyDependencies struct {
	Repository           ports.AuthAttemptRepository
	SubjectFingerprinter attemptFingerprinter
	IPFingerprinter      attemptFingerprinter
	Now                  func() time.Time
	Config               ports.AuthAttemptPolicy
}

type AttemptPolicy struct {
	repository           ports.AuthAttemptRepository
	subjectFingerprinter attemptFingerprinter
	ipFingerprinter      attemptFingerprinter
	now                  func() time.Time
	config               ports.AuthAttemptPolicy
}

func NewAttemptPolicy(dependencies AttemptPolicyDependencies) (*AttemptPolicy, error) {
	if dependencies.Repository == nil ||
		dependencies.SubjectFingerprinter == nil ||
		dependencies.IPFingerprinter == nil ||
		dependencies.Now == nil ||
		dependencies.Config.Threshold < 2 ||
		dependencies.Config.Window <= 0 ||
		dependencies.Config.LockDuration <= 0 {
		return nil, ErrInvalidAttemptPolicy
	}
	return &AttemptPolicy{
		repository:           dependencies.Repository,
		subjectFingerprinter: dependencies.SubjectFingerprinter,
		ipFingerprinter:      dependencies.IPFingerprinter,
		now:                  dependencies.Now,
		config:               dependencies.Config,
	}, nil
}

func (policy *AttemptPolicy) Check(
	ctx context.Context,
	email string,
	clientIP string,
) ([]ports.AuthAttemptKey, error) {
	normalizedIP, err := normalizeClientIP(clientIP)
	if err != nil {
		return nil, err
	}
	keys := []ports.AuthAttemptKey{
		{
			Type: ports.AuthAttemptKeySubject,
			Hash: policy.subjectFingerprinter.Sum(identitydomain.NormalizeEmail(email)),
		},
		{
			Type: ports.AuthAttemptKeyIP,
			Hash: policy.ipFingerprinter.Sum(normalizedIP),
		},
	}

	now := policy.now()
	decision, err := policy.repository.Check(ctx, keys, now)
	if err != nil {
		return nil, err
	}
	if decision.LockedUntil != nil && now.Before(*decision.LockedUntil) {
		return nil, ErrAuthenticationTemporarilyLocked
	}
	return keys, nil
}

func (policy *AttemptPolicy) RecordFailure(
	ctx context.Context,
	keys []ports.AuthAttemptKey,
) error {
	if !validAttemptKeys(keys) {
		return ErrInvalidAttemptPolicy
	}
	now := policy.now()
	decision, err := policy.repository.RecordFailure(ctx, keys, now, policy.config)
	if err != nil {
		return err
	}
	if decision.LockedUntil != nil && now.Before(*decision.LockedUntil) {
		return ErrAuthenticationTemporarilyLocked
	}
	return nil
}

func (policy *AttemptPolicy) RecordSuccess(
	ctx context.Context,
	keys []ports.AuthAttemptKey,
) error {
	if !validAttemptKeys(keys) {
		return ErrInvalidAttemptPolicy
	}
	return policy.repository.Reset(ctx, keys[0])
}

func normalizeClientIP(value string) (string, error) {
	address, err := netip.ParseAddr(value)
	if err != nil {
		return "", ErrInvalidClientIP
	}
	return address.Unmap().String(), nil
}

func validAttemptKeys(keys []ports.AuthAttemptKey) bool {
	return len(keys) == 2 &&
		keys[0].Type == ports.AuthAttemptKeySubject &&
		keys[1].Type == ports.AuthAttemptKeyIP
}
