package domain

import "time"

type OneTimeTokenPurpose string

const (
	OneTimeTokenPurposeUserActivation OneTimeTokenPurpose = "user_activation"
	OneTimeTokenPurposePasswordReset  OneTimeTokenPurpose = "password_reset"
)

type OneTimeToken struct {
	ID             string
	UserID         string
	Purpose        OneTimeTokenPurpose
	TokenHash      TokenHash
	HashVersion    int16
	HashKeyVersion int64
	ExpiresAt      time.Time
	CreatedAt      time.Time
}

type EncryptedTokenDelivery struct {
	TokenID    string
	UserID     string
	Recipient  string
	Purpose    OneTimeTokenPurpose
	KeyVersion int64
	Nonce      []byte
	Ciphertext []byte
	CreatedAt  time.Time
}
