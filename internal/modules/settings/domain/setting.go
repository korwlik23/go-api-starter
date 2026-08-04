package domain

import (
	"errors"
	"regexp"
	"strings"
)

var (
	ErrInvalidSetting  = errors.New("settings: invalid setting")
	ErrSecretSetting   = errors.New("settings: secret keys are not editable")
	ErrVersionConflict = errors.New("settings: version conflict")
)

var settingKeyPattern = regexp.MustCompile(`^[a-z][a-z0-9]*(?:[._-][a-z0-9]+)*$`)

type Setting struct {
	Key     string
	Value   string
	Version int64
}

func ValidateKey(value string) error {
	key := strings.TrimSpace(value)
	if key == "" || len(key) > 128 || !settingKeyPattern.MatchString(key) {
		return ErrInvalidSetting
	}
	if isSecretKey(key) {
		return ErrSecretSetting
	}
	return nil
}

func (setting Setting) Update(key, value string, expectedVersion int64) (Setting, error) {
	if setting.Version < 1 || expectedVersion < 1 || setting.Version != expectedVersion {
		return Setting{}, ErrVersionConflict
	}
	key = strings.TrimSpace(key)
	if err := ValidateKey(key); err != nil {
		return Setting{}, err
	}
	if len(value) > 16_384 {
		return Setting{}, ErrInvalidSetting
	}
	return Setting{Key: key, Value: value, Version: setting.Version + 1}, nil
}

func isSecretKey(key string) bool {
	for _, segment := range strings.FieldsFunc(key, func(r rune) bool { return r == '.' || r == '_' || r == '-' }) {
		switch strings.ToLower(segment) {
		case "password", "secret", "token", "private_key", "private", "credential":
			return true
		}
	}
	return false
}
