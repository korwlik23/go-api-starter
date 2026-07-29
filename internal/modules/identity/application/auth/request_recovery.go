package auth

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	identitydomain "github.com/korwlik23/go-api-starter/internal/modules/identity/domain"
	"github.com/korwlik23/go-api-starter/internal/modules/identity/ports"
	platformcrypto "github.com/korwlik23/go-api-starter/internal/platform/crypto"
)

var ErrInvalidRecoveryRequest = errors.New("invalid recovery request dependencies")

type recoveryTokenGenerator interface {
	Generate(
		purpose string,
		tokenID string,
		userID string,
	) (platformcrypto.BoundToken, error)
}

type recoveryDeliveryEncrypter interface {
	Encrypt(
		plaintext []byte,
		associatedData []byte,
	) (platformcrypto.Envelope, error)
}

type RequestRecoveryDependencies struct {
	Users      ports.UserRepository
	Tokens     recoveryTokenGenerator
	Delivery   recoveryDeliveryEncrypter
	Repository ports.OneTimeTokenIssuer
	NewID      func() (string, error)
	Now        func() time.Time
	TTL        time.Duration
}

type RequestRecovery struct {
	dependencies RequestRecoveryDependencies
}

func NewRequestRecovery(
	dependencies RequestRecoveryDependencies,
) (*RequestRecovery, error) {
	if dependencies.Users == nil ||
		dependencies.Tokens == nil ||
		dependencies.Delivery == nil ||
		dependencies.Repository == nil ||
		dependencies.NewID == nil ||
		dependencies.Now == nil ||
		dependencies.TTL <= 0 {
		return nil, ErrInvalidRecoveryRequest
	}
	return &RequestRecovery{dependencies: dependencies}, nil
}

func (useCase *RequestRecovery) Execute(ctx context.Context, email string) error {
	normalizedEmail := identitydomain.NormalizeEmail(email)
	user, err := useCase.dependencies.Users.FindByNormalizedEmail(ctx, normalizedEmail)
	if err != nil && !errors.Is(err, ports.ErrUserNotFound) {
		return err
	}
	eligible := err == nil && user.Status == identitydomain.UserStatusActive

	tokenID, err := useCase.dependencies.NewID()
	if err != nil {
		return err
	}
	tokenUserID := user.ID
	if tokenUserID == "" {
		tokenUserID = normalizedEmail
	}
	token, err := useCase.dependencies.Tokens.Generate(
		string(identitydomain.OneTimeTokenPurposePasswordReset),
		tokenID,
		tokenUserID,
	)
	if err != nil {
		return err
	}

	if !eligible {
		return nil
	}

	now := useCase.dependencies.Now()
	associatedData := recoveryDeliveryAssociatedData(tokenID, user.ID)
	payload, err := json.Marshal(struct {
		Recipient string `json:"recipient"`
		Token     string `json:"token"`
	}{
		Recipient: user.Email,
		Token:     token.Raw,
	})
	if err != nil {
		return err
	}
	envelope, err := useCase.dependencies.Delivery.Encrypt(
		payload,
		associatedData,
	)
	if err != nil {
		return err
	}

	record := identitydomain.OneTimeToken{
		ID:             tokenID,
		UserID:         user.ID,
		Purpose:        identitydomain.OneTimeTokenPurposePasswordReset,
		TokenHash:      identitydomain.TokenHash(token.Hash),
		HashVersion:    token.HashVersion,
		HashKeyVersion: token.HashKeyVersion,
		ExpiresAt:      now.Add(useCase.dependencies.TTL),
		CreatedAt:      now,
	}
	delivery := identitydomain.EncryptedTokenDelivery{
		TokenID:    tokenID,
		KeyVersion: envelope.KeyVersion,
		Nonce:      append([]byte(nil), envelope.Nonce...),
		Ciphertext: append([]byte(nil), envelope.Ciphertext...),
		ExpiresAt:  now.Add(useCase.dependencies.TTL),
		CreatedAt:  now,
	}
	return useCase.dependencies.Repository.Issue(ctx, record, delivery)
}

func recoveryDeliveryAssociatedData(tokenID, userID string) []byte {
	return []byte(strings.Join(
		[]string{"identity.password-recovery-delivery.v1", tokenID, userID},
		"\x00",
	))
}
