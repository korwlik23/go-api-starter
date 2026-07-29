package config

import (
	"encoding/base64"
	"errors"
	"strconv"
	"strings"
)

var ErrInvalidTOTPKeyring = errors.New("invalid TOTP encryption keyring")

type TOTPKeyring struct {
	active int64
	keys   map[int64][]byte
}

func ParseTOTPKeyring(activeVersion int64, serialized string) (TOTPKeyring, error) {
	if activeVersion < 1 || strings.TrimSpace(serialized) == "" {
		return TOTPKeyring{}, ErrInvalidTOTPKeyring
	}

	keys := make(map[int64][]byte)
	for entry := range strings.SplitSeq(serialized, ",") {
		parts := strings.Split(strings.TrimSpace(entry), ":")
		if len(parts) != 2 {
			return TOTPKeyring{}, ErrInvalidTOTPKeyring
		}
		version, err := strconv.ParseInt(parts[0], 10, 64)
		if err != nil || version < 1 {
			return TOTPKeyring{}, ErrInvalidTOTPKeyring
		}
		if _, duplicate := keys[version]; duplicate {
			return TOTPKeyring{}, ErrInvalidTOTPKeyring
		}
		decoded, err := base64.StdEncoding.Strict().DecodeString(parts[1])
		if err != nil || len(decoded) != 32 {
			return TOTPKeyring{}, ErrInvalidTOTPKeyring
		}
		keys[version] = append([]byte(nil), decoded...)
	}
	if _, ok := keys[activeVersion]; !ok {
		return TOTPKeyring{}, ErrInvalidTOTPKeyring
	}

	return TOTPKeyring{active: activeVersion, keys: keys}, nil
}

func (keyring TOTPKeyring) Active() (int64, []byte) {
	key := keyring.keys[keyring.active]
	return keyring.active, append([]byte(nil), key...)
}

func (keyring TOTPKeyring) Key(version int64) ([]byte, bool) {
	key, ok := keyring.keys[version]
	if !ok {
		return nil, false
	}
	return append([]byte(nil), key...), true
}
