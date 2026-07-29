package crypto

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"io"
	"strings"
)

const (
	TokenEntropyBytes = 32
	minimumTokenKey   = 32
)

var (
	ErrTokenKeyTooShort    = errors.New("token key must contain at least 32 bytes")
	ErrTokenDomainRequired = errors.New("token domain is required")
)

type OpaqueToken struct {
	Raw  string
	Hash [sha256.Size]byte
}

type TokenCodec struct {
	key    []byte
	domain string
	random io.Reader
}

func NewTokenCodec(key []byte, domain string) (*TokenCodec, error) {
	return newTokenCodec(key, domain, rand.Reader)
}

func newTokenCodec(key []byte, domain string, random io.Reader) (*TokenCodec, error) {
	if len(key) < minimumTokenKey {
		return nil, ErrTokenKeyTooShort
	}
	if strings.TrimSpace(domain) == "" {
		return nil, ErrTokenDomainRequired
	}

	keyCopy := make([]byte, len(key))
	copy(keyCopy, key)

	return &TokenCodec{
		key:    keyCopy,
		domain: domain,
		random: random,
	}, nil
}

func (codec *TokenCodec) Generate() (OpaqueToken, error) {
	randomBytes := make([]byte, TokenEntropyBytes)
	if _, err := io.ReadFull(codec.random, randomBytes); err != nil {
		return OpaqueToken{}, err
	}

	raw := base64.RawURLEncoding.EncodeToString(randomBytes)
	return OpaqueToken{
		Raw:  raw,
		Hash: codec.Hash(raw),
	}, nil
}

func (codec *TokenCodec) Hash(raw string) [sha256.Size]byte {
	mac := hmac.New(sha256.New, codec.key)
	_, _ = mac.Write([]byte(codec.domain))
	_, _ = mac.Write([]byte{0})
	_, _ = mac.Write([]byte(raw))

	var hash [sha256.Size]byte
	copy(hash[:], mac.Sum(nil))
	return hash
}

func (codec *TokenCodec) Matches(raw string, expected [sha256.Size]byte) bool {
	actual := codec.Hash(raw)
	return hmac.Equal(actual[:], expected[:])
}
