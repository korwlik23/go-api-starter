package config

import (
	"encoding/base64"
	"errors"
	"regexp"
	"strings"
)

var ErrInvalidTOTPKeyring = errors.New("invalid TOTP encryption keyring")

var keyVersionPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{0,31}$`)

type TOTPKeyring struct {
	active string
	keys   map[string][]byte
}

func ParseTOTPKeyring(activeVersion, serialized string) (TOTPKeyring, error) {
	activeVersion = strings.TrimSpace(activeVersion)
	if !keyVersionPattern.MatchString(activeVersion) || strings.TrimSpace(serialized) == "" {
		return TOTPKeyring{}, ErrInvalidTOTPKeyring
	}

	keys := make(map[string][]byte)
	for entry := range strings.SplitSeq(serialized, ",") {
		parts := strings.Split(strings.TrimSpace(entry), ":")
		if len(parts) != 2 || !keyVersionPattern.MatchString(parts[0]) {
			return TOTPKeyring{}, ErrInvalidTOTPKeyring
		}
		if _, duplicate := keys[parts[0]]; duplicate {
			return TOTPKeyring{}, ErrInvalidTOTPKeyring
		}
		decoded, err := base64.StdEncoding.Strict().DecodeString(parts[1])
		if err != nil || len(decoded) != 32 {
			return TOTPKeyring{}, ErrInvalidTOTPKeyring
		}
		keys[parts[0]] = append([]byte(nil), decoded...)
	}
	if _, ok := keys[activeVersion]; !ok {
		return TOTPKeyring{}, ErrInvalidTOTPKeyring
	}

	return TOTPKeyring{active: activeVersion, keys: keys}, nil
}

func (keyring TOTPKeyring) Active() (string, []byte) {
	key := keyring.keys[keyring.active]
	return keyring.active, append([]byte(nil), key...)
}

func (keyring TOTPKeyring) Key(version string) ([]byte, bool) {
	key, ok := keyring.keys[version]
	if !ok {
		return nil, false
	}
	return append([]byte(nil), key...), true
}
