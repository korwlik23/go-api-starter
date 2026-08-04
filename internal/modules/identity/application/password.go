package application

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/korwlik23/go-api-starter/internal/modules/identity/ports"
)

var (
	ErrInvalidPasswordChange  = errors.New("invalid password change")
	ErrCurrentPasswordInvalid = errors.New("current password is invalid")
)

type PasswordVerifier interface {
	Verify(password, encodedHash string) (bool, error)
}

type PasswordHasher interface {
	Hash(password string) (string, error)
}

type PasswordChangeWriter interface {
	ChangePasswordAndInvalidate(context.Context, string, string, time.Time) error
}

type ChangePasswordDependencies struct {
	Passwords ports.PasswordRepository
	Writer    PasswordChangeWriter
	Verifier  PasswordVerifier
	Hasher    PasswordHasher
	Now       func() time.Time
}

type ChangePasswordService struct {
	passwords ports.PasswordRepository
	writer    PasswordChangeWriter
	verifier  PasswordVerifier
	hasher    PasswordHasher
	now       func() time.Time
}

func NewChangePasswordService(dependencies ChangePasswordDependencies) (*ChangePasswordService, error) {
	if dependencies.Passwords == nil || dependencies.Writer == nil || dependencies.Verifier == nil || dependencies.Hasher == nil || dependencies.Now == nil {
		return nil, ErrInvalidPasswordChange
	}
	return &ChangePasswordService{
		passwords: dependencies.Passwords,
		writer:    dependencies.Writer,
		verifier:  dependencies.Verifier,
		hasher:    dependencies.Hasher,
		now:       dependencies.Now,
	}, nil
}

func (service *ChangePasswordService) Execute(ctx context.Context, userID, currentPassword, newPassword string) error {
	if service == nil || ctx == nil || strings.TrimSpace(userID) == "" || strings.TrimSpace(currentPassword) == "" || strings.TrimSpace(newPassword) == "" || currentPassword == newPassword {
		return ErrInvalidPasswordChange
	}
	encodedHash, err := service.passwords.FindPasswordHash(ctx, userID)
	if err != nil {
		return err
	}
	matched, err := service.verifier.Verify(currentPassword, encodedHash)
	if err != nil {
		return err
	}
	if !matched {
		return ErrCurrentPasswordInvalid
	}
	newHash, err := service.hasher.Hash(newPassword)
	if err != nil {
		return err
	}
	if err := service.writer.ChangePasswordAndInvalidate(ctx, userID, newHash, service.now().UTC()); err != nil {
		return err
	}
	return nil
}

var _ PasswordChangeWriter = (interface {
	ChangePasswordAndInvalidate(context.Context, string, string, time.Time) error
})(nil)
