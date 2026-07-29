package domain

import "errors"

var ErrInvalidTokenHash = errors.New("token hash must contain exactly 32 bytes")

type TokenHash [32]byte

func NewTokenHash(value []byte) (TokenHash, error) {
	if len(value) != len(TokenHash{}) {
		return TokenHash{}, ErrInvalidTokenHash
	}

	var hash TokenHash
	copy(hash[:], value)
	return hash, nil
}

type Session struct {
	ID        string
	TokenHash TokenHash
}
