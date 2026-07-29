package config

import (
	"encoding/base64"
	"errors"
	"strconv"
	"strings"
)

var (
	ErrInvalidRecoveryTokenKeyring = errors.New(
		"invalid recovery token HMAC keyring",
	)
	ErrInvalidRecoveryDeliveryKeyring = errors.New(
		"invalid recovery delivery encryption keyring",
	)
)

type RecoveryTokenKeyring struct {
	versionedKeyring
}

type RecoveryDeliveryKeyring struct {
	versionedKeyring
}

type versionedKeyring struct {
	active int64
	keys   map[int64][]byte
}

func ParseRecoveryTokenKeyring(
	activeVersion int64,
	serialized string,
) (RecoveryTokenKeyring, error) {
	keyring, err := parseVersionedKeyring(activeVersion, serialized)
	if err != nil {
		return RecoveryTokenKeyring{}, ErrInvalidRecoveryTokenKeyring
	}
	return RecoveryTokenKeyring{versionedKeyring: keyring}, nil
}

func ParseRecoveryDeliveryKeyring(
	activeVersion int64,
	serialized string,
) (RecoveryDeliveryKeyring, error) {
	keyring, err := parseVersionedKeyring(activeVersion, serialized)
	if err != nil {
		return RecoveryDeliveryKeyring{}, ErrInvalidRecoveryDeliveryKeyring
	}
	return RecoveryDeliveryKeyring{versionedKeyring: keyring}, nil
}

func parseVersionedKeyring(
	activeVersion int64,
	serialized string,
) (versionedKeyring, error) {
	if activeVersion < 1 || strings.TrimSpace(serialized) == "" {
		return versionedKeyring{}, errors.New("invalid keyring")
	}
	keys := make(map[int64][]byte)
	for entry := range strings.SplitSeq(serialized, ",") {
		parts := strings.Split(strings.TrimSpace(entry), ":")
		if len(parts) != 2 {
			return versionedKeyring{}, errors.New("invalid keyring entry")
		}
		version, err := strconv.ParseInt(parts[0], 10, 64)
		if err != nil || version < 1 {
			return versionedKeyring{}, errors.New("invalid key version")
		}
		if _, duplicate := keys[version]; duplicate {
			return versionedKeyring{}, errors.New("duplicate key version")
		}
		key, err := base64.StdEncoding.Strict().DecodeString(parts[1])
		if err != nil || len(key) != 32 {
			return versionedKeyring{}, errors.New("invalid key material")
		}
		keys[version] = append([]byte(nil), key...)
	}
	if _, ok := keys[activeVersion]; !ok {
		return versionedKeyring{}, errors.New("active key missing")
	}
	return versionedKeyring{active: activeVersion, keys: keys}, nil
}

func (keyring versionedKeyring) Active() (int64, []byte) {
	key := keyring.keys[keyring.active]
	return keyring.active, append([]byte(nil), key...)
}

func (keyring versionedKeyring) Key(version int64) ([]byte, bool) {
	key, ok := keyring.keys[version]
	if !ok {
		return nil, false
	}
	return append([]byte(nil), key...), true
}
