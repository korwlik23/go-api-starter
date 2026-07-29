package ports

import (
	"context"
	"errors"
	"time"

	"github.com/korwlik23/go-api-starter/internal/modules/identity/domain"
)

var (
	ErrOneTimeTokenNotFound = errors.New("one-time token not found")
	ErrOneTimeTokenConflict = errors.New("one-time token conflict")
)

type OneTimeTokenIssuer interface {
	Issue(
		ctx context.Context,
		token domain.OneTimeToken,
		delivery domain.EncryptedTokenDelivery,
	) error
}

type PasswordResetTokenMetadata struct {
	ID                   string
	UserID               string
	HashVersion          int16
	HashKeyVersion       int64
	AuthorizationVersion int64
	MFARequired          bool
}

type CompletePasswordResetCommand struct {
	TokenID            string
	CandidateHash      domain.TokenHash
	HashVersion        int16
	NewPasswordHash    string
	CurrentSessionHash domain.TokenHash
	Replacement        domain.Session
	Now                time.Time
}

type PasswordResetRepository interface {
	FindPasswordResetToken(
		ctx context.Context,
		tokenID string,
	) (PasswordResetTokenMetadata, error)
	CompletePasswordReset(
		ctx context.Context,
		command CompletePasswordResetCommand,
	) error
}
