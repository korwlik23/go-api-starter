package auth

import (
	"context"
	"errors"

	sessionapp "github.com/korwlik23/go-api-starter/internal/modules/identity/application/session"
	identitydomain "github.com/korwlik23/go-api-starter/internal/modules/identity/domain"
	"github.com/korwlik23/go-api-starter/internal/modules/identity/ports"
)

type LoginStatus string

const (
	LoginStatusAuthenticated LoginStatus = "authenticated"
	LoginStatusMFAPending    LoginStatus = "mfa_pending"
)

var ErrInvalidLoginFlow = errors.New("invalid login flow dependencies")

type loginPasswordAuthenticator interface {
	Authenticate(
		ctx context.Context,
		email string,
		password string,
		clientIP string,
	) (identitydomain.User, error)
}

type loginSessionRotator interface {
	Authenticated(
		ctx context.Context,
		currentHash identitydomain.TokenHash,
		userID string,
		authorizationVersion int64,
	) (sessionapp.RotationResult, error)
	MFAPending(
		ctx context.Context,
		currentHash identitydomain.TokenHash,
		userID string,
		authorizationVersion int64,
	) (sessionapp.RotationResult, error)
}

type LoginCommand struct {
	Email              string
	Password           string
	ClientIP           string
	CurrentSessionHash identitydomain.TokenHash
}

type LoginResult struct {
	Status       LoginStatus
	ChallengeID  string
	SessionToken string
	CSRFToken    string
}

type LoginFlow struct {
	authenticator loginPasswordAuthenticator
	mfa           ports.MFAStatusReader
	rotator       loginSessionRotator
}

func NewLoginFlow(
	authenticator loginPasswordAuthenticator,
	mfa ports.MFAStatusReader,
	rotator loginSessionRotator,
) (*LoginFlow, error) {
	if authenticator == nil || mfa == nil || rotator == nil {
		return nil, ErrInvalidLoginFlow
	}
	return &LoginFlow{authenticator: authenticator, mfa: mfa, rotator: rotator}, nil
}

func (flow *LoginFlow) Execute(
	ctx context.Context,
	command LoginCommand,
) (LoginResult, error) {
	user, err := flow.authenticator.Authenticate(
		ctx,
		command.Email,
		command.Password,
		command.ClientIP,
	)
	if err != nil {
		return LoginResult{}, err
	}
	if user.ID == "" || user.AuthorizationVersion < 1 {
		return LoginResult{}, ErrInvalidLoginFlow
	}

	mfaEnabled, err := flow.mfa.IsEnabled(ctx, user.ID)
	if err != nil {
		return LoginResult{}, err
	}

	var rotation sessionapp.RotationResult
	status := LoginStatusAuthenticated
	if mfaEnabled {
		status = LoginStatusMFAPending
		rotation, err = flow.rotator.MFAPending(
			ctx,
			command.CurrentSessionHash,
			user.ID,
			user.AuthorizationVersion,
		)
	} else {
		rotation, err = flow.rotator.Authenticated(
			ctx,
			command.CurrentSessionHash,
			user.ID,
			user.AuthorizationVersion,
		)
	}
	if err != nil {
		return LoginResult{}, err
	}

	result := LoginResult{
		Status:       status,
		SessionToken: rotation.SessionToken,
		CSRFToken:    rotation.CSRFToken,
	}
	if status == LoginStatusMFAPending {
		result.ChallengeID = rotation.SessionID
	}
	return result, nil
}
