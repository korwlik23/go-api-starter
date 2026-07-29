package crypto

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"io"
	"strings"
)

const BoundTokenHashVersion int16 = 1

var ErrInvalidBoundTokenCodec = errors.New("invalid bound token codec configuration")

type BoundToken struct {
	Raw            string
	Hash           [sha256.Size]byte
	HashVersion    int16
	HashKeyVersion int64
}

type boundTokenKeyring interface {
	Active() (version int64, key []byte)
	Key(version int64) ([]byte, bool)
}

type BoundTokenCodec struct {
	keyring boundTokenKeyring
	random  io.Reader
}

func NewBoundTokenCodec(keyring boundTokenKeyring) (*BoundTokenCodec, error) {
	return newBoundTokenCodec(keyring, rand.Reader)
}

func newBoundTokenCodec(
	keyring boundTokenKeyring,
	random io.Reader,
) (*BoundTokenCodec, error) {
	if keyring == nil || random == nil {
		return nil, ErrInvalidBoundTokenCodec
	}
	version, key := keyring.Active()
	if version < 1 || len(key) < minimumTokenKey {
		return nil, ErrInvalidBoundTokenCodec
	}
	return &BoundTokenCodec{keyring: keyring, random: random}, nil
}

func (codec *BoundTokenCodec) Generate(
	purpose string,
	tokenID string,
	userID string,
) (BoundToken, error) {
	if !validBoundTokenFields(purpose, tokenID, userID) ||
		strings.Contains(tokenID, ".") {
		return BoundToken{}, ErrInvalidBoundTokenCodec
	}

	entropy := make([]byte, TokenEntropyBytes)
	if _, err := io.ReadFull(codec.random, entropy); err != nil {
		return BoundToken{}, err
	}
	raw := tokenID + "." + base64.RawURLEncoding.EncodeToString(entropy)
	keyVersion, _ := codec.keyring.Active()
	hash := codec.Hash(purpose, tokenID, userID, raw, keyVersion)
	return BoundToken{
		Raw:            raw,
		Hash:           hash,
		HashVersion:    BoundTokenHashVersion,
		HashKeyVersion: keyVersion,
	}, nil
}

func (codec *BoundTokenCodec) Hash(
	purpose string,
	tokenID string,
	userID string,
	raw string,
	keyVersion int64,
) [sha256.Size]byte {
	key, ok := codec.keyring.Key(keyVersion)
	if !ok || len(key) < minimumTokenKey {
		return [sha256.Size]byte{}
	}

	mac := hmac.New(sha256.New, key)
	writeBoundTokenField(mac, "identity.bound-token.v1")
	writeBoundTokenField(mac, purpose)
	writeBoundTokenField(mac, tokenID)
	writeBoundTokenField(mac, userID)
	writeBoundTokenField(mac, raw)

	var hash [sha256.Size]byte
	copy(hash[:], mac.Sum(nil))
	return hash
}

func (codec *BoundTokenCodec) Matches(
	purpose string,
	tokenID string,
	userID string,
	raw string,
	keyVersion int64,
	expected [sha256.Size]byte,
) bool {
	if !validBoundTokenFields(purpose, tokenID, userID) || raw == "" {
		return false
	}
	key, ok := codec.keyring.Key(keyVersion)
	if !ok || len(key) < minimumTokenKey {
		return false
	}
	actual := codec.Hash(purpose, tokenID, userID, raw, keyVersion)
	return hmac.Equal(actual[:], expected[:])
}

func ParseBoundToken(raw string) (string, bool) {
	tokenID, secret, ok := strings.Cut(raw, ".")
	if !ok || tokenID == "" || secret == "" || strings.Contains(secret, ".") {
		return "", false
	}
	entropy, err := base64.RawURLEncoding.Strict().DecodeString(secret)
	if err != nil || len(entropy) != TokenEntropyBytes {
		return "", false
	}
	return tokenID, true
}

func (codec *BoundTokenCodec) Parse(raw string) (string, bool) {
	return ParseBoundToken(raw)
}

func validBoundTokenFields(purpose, tokenID, userID string) bool {
	return strings.TrimSpace(purpose) != "" &&
		strings.TrimSpace(tokenID) != "" &&
		strings.TrimSpace(userID) != ""
}

func writeBoundTokenField(writer io.Writer, value string) {
	var length [4]byte
	binary.BigEndian.PutUint32(length[:], uint32(len(value)))
	_, _ = writer.Write(length[:])
	_, _ = io.WriteString(writer, value)
}
