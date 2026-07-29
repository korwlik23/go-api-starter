package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"errors"
	"io"
)

const EnvelopeNonceBytes = 12

var (
	ErrInvalidEnvelopeCipher  = errors.New("invalid envelope cipher configuration")
	ErrEnvelopeAuthentication = errors.New("envelope authentication failed")
)

type Envelope struct {
	KeyVersion int64
	Nonce      []byte
	Ciphertext []byte
}

type envelopeKeyring interface {
	Active() (version int64, key []byte)
	Key(version int64) ([]byte, bool)
}

type EnvelopeCipher struct {
	keyring envelopeKeyring
	random  io.Reader
}

func NewEnvelopeCipher(keyring envelopeKeyring) (*EnvelopeCipher, error) {
	return newEnvelopeCipher(keyring, rand.Reader)
}

func newEnvelopeCipher(
	keyring envelopeKeyring,
	random io.Reader,
) (*EnvelopeCipher, error) {
	if keyring == nil || random == nil {
		return nil, ErrInvalidEnvelopeCipher
	}
	version, key := keyring.Active()
	if version < 1 || len(key) != 32 {
		return nil, ErrInvalidEnvelopeCipher
	}
	return &EnvelopeCipher{keyring: keyring, random: random}, nil
}

func (envelopeCipher *EnvelopeCipher) Encrypt(
	plaintext []byte,
	associatedData []byte,
) (Envelope, error) {
	version, key := envelopeCipher.keyring.Active()
	aead, err := newGCM(key)
	if err != nil || version < 1 {
		return Envelope{}, ErrInvalidEnvelopeCipher
	}

	nonce := make([]byte, EnvelopeNonceBytes)
	if _, err := io.ReadFull(envelopeCipher.random, nonce); err != nil {
		return Envelope{}, err
	}
	ciphertext := aead.Seal(nil, nonce, plaintext, associatedData)
	return Envelope{
		KeyVersion: version,
		Nonce:      nonce,
		Ciphertext: ciphertext,
	}, nil
}

func (envelopeCipher *EnvelopeCipher) Decrypt(
	envelope Envelope,
	associatedData []byte,
) ([]byte, error) {
	key, ok := envelopeCipher.keyring.Key(envelope.KeyVersion)
	if !ok || len(envelope.Nonce) != EnvelopeNonceBytes {
		return nil, ErrEnvelopeAuthentication
	}
	aead, err := newGCM(key)
	if err != nil {
		return nil, ErrEnvelopeAuthentication
	}
	plaintext, err := aead.Open(nil, envelope.Nonce, envelope.Ciphertext, associatedData)
	if err != nil {
		return nil, ErrEnvelopeAuthentication
	}
	return plaintext, nil
}

func newGCM(key []byte) (cipher.AEAD, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	return cipher.NewGCMWithNonceSize(block, EnvelopeNonceBytes)
}
