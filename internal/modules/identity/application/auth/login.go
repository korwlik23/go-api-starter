package auth

import (
	"context"
	"errors"

	identitydomain "github.com/korwlik23/go-api-starter/internal/modules/identity/domain"
	"github.com/korwlik23/go-api-starter/internal/modules/identity/ports"
)

var (
	ErrInvalidCredentials   = errors.New("invalid credentials")
	ErrInvalidPasswordLogin = errors.New("invalid password login dependencies")
)

type PasswordLoginDependencies struct {
	Users     ports.UserRepository
	Passwords ports.PasswordRepository
	Verifier  ports.PasswordVerifier
	Attempts  loginAttemptPolicy
	DummyHash string
}

type PasswordLogin struct {
	dependencies PasswordLoginDependencies
}

type loginAttemptPolicy interface {
	Check(ctx context.Context, email, clientIP string) ([]ports.AuthAttemptKey, error)
	RecordFailure(ctx context.Context, keys []ports.AuthAttemptKey) error
	RecordSuccess(ctx context.Context, keys []ports.AuthAttemptKey) error
}

func NewPasswordLogin(dependencies PasswordLoginDependencies) (*PasswordLogin, error) {
	if dependencies.Users == nil ||
		dependencies.Passwords == nil ||
		dependencies.Verifier == nil ||
		dependencies.Attempts == nil ||
		dependencies.DummyHash == "" {
		return nil, ErrInvalidPasswordLogin
	}
	return &PasswordLogin{dependencies: dependencies}, nil
}

func (login *PasswordLogin) Authenticate(
	ctx context.Context,
	email string,
	password string,
	clientIP string,
) (identitydomain.User, error) {
	attemptKeys, err := login.dependencies.Attempts.Check(ctx, email, clientIP)
	if errors.Is(err, ErrAuthenticationTemporarilyLocked) {
		if _, verifyErr := login.dependencies.Verifier.Verify(
			password,
			login.dependencies.DummyHash,
		); verifyErr != nil {
			return identitydomain.User{}, verifyErr
		}
		return identitydomain.User{}, ErrInvalidCredentials
	}
	if err != nil {
		return identitydomain.User{}, err
	}

	user, err := login.dependencies.Users.FindByNormalizedEmail(
		ctx,
		identitydomain.NormalizeEmail(email),
	)
	if errors.Is(err, ports.ErrUserNotFound) {
		return identitydomain.User{}, login.rejectWithDummyVerification(
			ctx,
			password,
			attemptKeys,
		)
	}
	if err != nil {
		return identitydomain.User{}, err
	}
	if user.Status != identitydomain.UserStatusActive {
		return identitydomain.User{}, login.rejectWithDummyVerification(
			ctx,
			password,
			attemptKeys,
		)
	}

	hash, err := login.dependencies.Passwords.FindPasswordHash(ctx, user.ID)
	if errors.Is(err, ports.ErrPasswordNotFound) {
		return identitydomain.User{}, login.rejectWithDummyVerification(
			ctx,
			password,
			attemptKeys,
		)
	}
	if err != nil {
		return identitydomain.User{}, err
	}

	matches, err := login.dependencies.Verifier.Verify(password, hash)
	if err != nil {
		return identitydomain.User{}, err
	}
	if !matches {
		return identitydomain.User{}, login.recordFailure(ctx, attemptKeys)
	}
	if err := login.dependencies.Attempts.RecordSuccess(ctx, attemptKeys); err != nil {
		return identitydomain.User{}, err
	}
	return user, nil
}

func (login *PasswordLogin) rejectWithDummyVerification(
	ctx context.Context,
	password string,
	attemptKeys []ports.AuthAttemptKey,
) error {
	if _, err := login.dependencies.Verifier.Verify(password, login.dependencies.DummyHash); err != nil {
		return err
	}
	return login.recordFailure(ctx, attemptKeys)
}

func (login *PasswordLogin) recordFailure(
	ctx context.Context,
	attemptKeys []ports.AuthAttemptKey,
) error {
	err := login.dependencies.Attempts.RecordFailure(ctx, attemptKeys)
	if err != nil && !errors.Is(err, ErrAuthenticationTemporarilyLocked) {
		return err
	}
	return ErrInvalidCredentials
}
