package crypto

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base32"
	"encoding/binary"
	"errors"
	"io"
	"strings"
)

const (
	RecoveryCodeEntropyBytes = 16
	RecoveryCodeHashVersion  = int16(1)
)

var ErrInvalidRecoveryCodeCodec = errors.New(
	"invalid MFA recovery code codec configuration",
)

type RecoveryCode struct {
	Raw            string
	Hash           [sha256.Size]byte
	HashVersion    int16
	HashKeyVersion int64
}

type RecoveryCodeCodec struct {
	keyring boundTokenKeyring
	random  io.Reader
}

func NewRecoveryCodeCodec(
	keyring boundTokenKeyring,
) (*RecoveryCodeCodec, error) {
	return newRecoveryCodeCodec(keyring, rand.Reader)
}

func newRecoveryCodeCodec(
	keyring boundTokenKeyring,
	random io.Reader,
) (*RecoveryCodeCodec, error) {
	if keyring == nil || random == nil {
		return nil, ErrInvalidRecoveryCodeCodec
	}
	version, key := keyring.Active()
	if version < 1 || len(key) < minimumTokenKey {
		return nil, ErrInvalidRecoveryCodeCodec
	}
	return &RecoveryCodeCodec{keyring: keyring, random: random}, nil
}

func (codec *RecoveryCodeCodec) Generate(
	userID string,
) (RecoveryCode, error) {
	if strings.TrimSpace(userID) == "" {
		return RecoveryCode{}, ErrInvalidRecoveryCodeCodec
	}
	entropy := make([]byte, RecoveryCodeEntropyBytes)
	if _, err := io.ReadFull(codec.random, entropy); err != nil {
		return RecoveryCode{}, err
	}
	encoded := base32.StdEncoding.WithPadding(base32.NoPadding).
		EncodeToString(entropy)
	raw := strings.ToLower(encoded[:13] + "-" + encoded[13:])
	version, _ := codec.keyring.Active()
	return RecoveryCode{
		Raw:            raw,
		Hash:           codec.Hash(userID, raw, version),
		HashVersion:    RecoveryCodeHashVersion,
		HashKeyVersion: version,
	}, nil
}

func (codec *RecoveryCodeCodec) Hash(
	userID string,
	raw string,
	keyVersion int64,
) [sha256.Size]byte {
	key, ok := codec.keyring.Key(keyVersion)
	if !ok || strings.TrimSpace(userID) == "" || strings.TrimSpace(raw) == "" {
		return [sha256.Size]byte{}
	}
	mac := hmac.New(sha256.New, key)
	writeRecoveryCodeField(mac, "identity.mfa-recovery-code.v1")
	writeRecoveryCodeField(mac, userID)
	writeRecoveryCodeField(mac, strings.ToLower(strings.TrimSpace(raw)))
	var result [sha256.Size]byte
	copy(result[:], mac.Sum(nil))
	return result
}

func (codec *RecoveryCodeCodec) Matches(
	userID string,
	raw string,
	keyVersion int64,
	expected [sha256.Size]byte,
) bool {
	actual := codec.Hash(userID, raw, keyVersion)
	return hmac.Equal(actual[:], expected[:])
}

func writeRecoveryCodeField(writer io.Writer, value string) {
	var length [4]byte
	binary.BigEndian.PutUint32(length[:], uint32(len(value)))
	_, _ = writer.Write(length[:])
	_, _ = io.WriteString(writer, value)
}
