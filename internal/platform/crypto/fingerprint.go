package crypto

import (
	"crypto/hmac"
	"crypto/sha256"
	"errors"
	"strings"
)

var (
	ErrFingerprintKeyTooShort    = errors.New("fingerprint key must contain at least 32 bytes")
	ErrFingerprintDomainRequired = errors.New("fingerprint domain is required")
)

type Fingerprinter struct {
	key    []byte
	domain string
}

func NewFingerprinter(key []byte, domain string) (*Fingerprinter, error) {
	if len(key) < 32 {
		return nil, ErrFingerprintKeyTooShort
	}
	if strings.TrimSpace(domain) == "" {
		return nil, ErrFingerprintDomainRequired
	}

	keyCopy := make([]byte, len(key))
	copy(keyCopy, key)
	return &Fingerprinter{key: keyCopy, domain: domain}, nil
}

func (fingerprinter *Fingerprinter) Sum(value string) [sha256.Size]byte {
	mac := hmac.New(sha256.New, fingerprinter.key)
	_, _ = mac.Write([]byte(fingerprinter.domain))
	_, _ = mac.Write([]byte{0})
	_, _ = mac.Write([]byte(value))

	var result [sha256.Size]byte
	copy(result[:], mac.Sum(nil))
	return result
}
