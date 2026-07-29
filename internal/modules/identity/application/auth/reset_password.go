package auth

import (
	"context"
	"errors"
	"strings"
	"time"

	identitydomain "github.com/korwlik23/go-api-starter/internal/modules/identity/domain"
	"github.com/korwlik23/go-api-starter/internal/modules/identity/ports"
	platformcrypto "github.com/korwlik23/go-api-starter/internal/platform/crypto"
)

var (
	ErrInvalidPasswordReset = errors.New("invalid password reset dependencies")
	ErrRecoveryTokenInvalid = errors.New("recovery token is invalid or expired")
)

type passwordResetTokenCodec interface {
	Parse(raw string) (tokenID string, ok bool)
	Hash(
		purpose string,
		tokenID string,
		userID string,
		raw string,
		keyVersion int64,
	) [32]byte
}

type passwordResetHasher interface {
	Hash(password string) (string, error)
}

type resetSessionTokenGenerator interface {
	Generate() (platformcrypto.OpaqueToken, error)
}

type ResetPasswordDependencies struct {
	Repository     ports.PasswordResetRepository
	Tokens         passwordResetTokenCodec
	Passwords      passwordResetHasher
	SessionTokens  resetSessionTokenGenerator
	CSRFTokens     resetSessionTokenGenerator
	NewID          func() (string, error)
	Now            func() time.Time
	IdleTTL        time.Duration
	AbsoluteTTL    time.Duration
	MFAIdleTTL     time.Duration
	MFAAbsoluteTTL time.Duration
}

type ResetPasswordCommand struct {
	RawToken           string
	NewPassword        string
	CurrentSessionHash identitydomain.TokenHash
}

type ResetPasswordResult struct {
	SessionToken string
	CSRFToken    string
}

type ResetPassword struct {
	dependencies ResetPasswordDependencies
}

func NewResetPassword(
	dependencies ResetPasswordDependencies,
) (*ResetPassword, error) {
	if dependencies.Repository == nil ||
		dependencies.Tokens == nil ||
		dependencies.Passwords == nil ||
		dependencies.SessionTokens == nil ||
		dependencies.CSRFTokens == nil ||
		dependencies.NewID == nil ||
		dependencies.Now == nil ||
		!validResetTTL(dependencies.IdleTTL, dependencies.AbsoluteTTL) ||
		!validResetTTL(dependencies.MFAIdleTTL, dependencies.MFAAbsoluteTTL) {
		return nil, ErrInvalidPasswordReset
	}
	return &ResetPassword{dependencies: dependencies}, nil
}

func (useCase *ResetPassword) Execute(
	ctx context.Context,
	command ResetPasswordCommand,
) (ResetPasswordResult, error) {
	tokenID, ok := useCase.dependencies.Tokens.Parse(command.RawToken)
	if !ok || strings.TrimSpace(command.NewPassword) == "" {
		return ResetPasswordResult{}, ErrRecoveryTokenInvalid
	}

	metadata, err := useCase.dependencies.Repository.FindPasswordResetToken(ctx, tokenID)
	if err != nil ||
		metadata.ID != tokenID ||
		metadata.UserID == "" ||
		metadata.HashVersion != platformcrypto.BoundTokenHashVersion ||
		metadata.HashKeyVersion < 1 ||
		metadata.AuthorizationVersion < 1 {
		return ResetPasswordResult{}, ErrRecoveryTokenInvalid
	}

	candidateHash := useCase.dependencies.Tokens.Hash(
		string(identitydomain.OneTimeTokenPurposePasswordReset),
		tokenID,
		metadata.UserID,
		command.RawToken,
		metadata.HashKeyVersion,
	)
	passwordHash, err := useCase.dependencies.Passwords.Hash(command.NewPassword)
	if err != nil {
		return ResetPasswordResult{}, err
	}
	sessionToken, err := useCase.dependencies.SessionTokens.Generate()
	if err != nil {
		return ResetPasswordResult{}, err
	}
	csrfToken, err := useCase.dependencies.CSRFTokens.Generate()
	if err != nil {
		return ResetPasswordResult{}, err
	}
	sessionID, err := useCase.dependencies.NewID()
	if err != nil {
		return ResetPasswordResult{}, err
	}

	now := useCase.dependencies.Now()
	state := identitydomain.SessionStateAuthenticated
	idleTTL := useCase.dependencies.IdleTTL
	absoluteTTL := useCase.dependencies.AbsoluteTTL
	if metadata.MFARequired {
		state = identitydomain.SessionStateMFAPending
		idleTTL = useCase.dependencies.MFAIdleTTL
		absoluteTTL = useCase.dependencies.MFAAbsoluteTTL
	}
	authorizationVersion := metadata.AuthorizationVersion + 1
	userID := metadata.UserID
	replacement := identitydomain.Session{
		ID:                   sessionID,
		TokenHash:            identitydomain.TokenHash(sessionToken.Hash),
		CSRFTokenHash:        identitydomain.TokenHash(csrfToken.Hash),
		State:                state,
		UserID:               &userID,
		AuthorizationVersion: &authorizationVersion,
		IdleExpiresAt:        now.Add(idleTTL),
		AbsoluteExpiresAt:    now.Add(absoluteTTL),
		CreatedAt:            now,
	}
	err = useCase.dependencies.Repository.CompletePasswordReset(
		ctx,
		ports.CompletePasswordResetCommand{
			TokenID:            tokenID,
			CandidateHash:      identitydomain.TokenHash(candidateHash),
			HashVersion:        metadata.HashVersion,
			NewPasswordHash:    passwordHash,
			CurrentSessionHash: command.CurrentSessionHash,
			Replacement:        replacement,
			Now:                now,
		},
	)
	if err != nil {
		return ResetPasswordResult{}, err
	}
	return ResetPasswordResult{
		SessionToken: sessionToken.Raw,
		CSRFToken:    csrfToken.Raw,
	}, nil
}

func validResetTTL(idle, absolute time.Duration) bool {
	return idle > 0 && absolute > 0 && idle <= absolute
}
