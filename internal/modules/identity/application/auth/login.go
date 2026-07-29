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
	DummyHash string
}

type PasswordLogin struct {
	dependencies PasswordLoginDependencies
}

func NewPasswordLogin(dependencies PasswordLoginDependencies) (*PasswordLogin, error) {
	if dependencies.Users == nil ||
		dependencies.Passwords == nil ||
		dependencies.Verifier == nil ||
		dependencies.DummyHash == "" {
		return nil, ErrInvalidPasswordLogin
	}
	return &PasswordLogin{dependencies: dependencies}, nil
}

func (login *PasswordLogin) Authenticate(
	ctx context.Context,
	email string,
	password string,
) (identitydomain.User, error) {
	user, err := login.dependencies.Users.FindByNormalizedEmail(
		ctx,
		identitydomain.NormalizeEmail(email),
	)
	if errors.Is(err, ports.ErrUserNotFound) {
		return identitydomain.User{}, login.rejectWithDummyVerification(password)
	}
	if err != nil {
		return identitydomain.User{}, err
	}
	if user.Status != identitydomain.UserStatusActive {
		return identitydomain.User{}, login.rejectWithDummyVerification(password)
	}

	hash, err := login.dependencies.Passwords.FindPasswordHash(ctx, user.ID)
	if errors.Is(err, ports.ErrPasswordNotFound) {
		return identitydomain.User{}, login.rejectWithDummyVerification(password)
	}
	if err != nil {
		return identitydomain.User{}, err
	}

	matches, err := login.dependencies.Verifier.Verify(password, hash)
	if err != nil {
		return identitydomain.User{}, err
	}
	if !matches {
		return identitydomain.User{}, ErrInvalidCredentials
	}
	return user, nil
}

func (login *PasswordLogin) rejectWithDummyVerification(password string) error {
	if _, err := login.dependencies.Verifier.Verify(password, login.dependencies.DummyHash); err != nil {
		return err
	}
	return ErrInvalidCredentials
}
