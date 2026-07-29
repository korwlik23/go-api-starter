package config

import (
	"encoding/base64"
	"strings"
)

type secretSetting struct {
	key   string
	value string
}

func validateSecrets(cfg Config) error {
	secrets := []secretSetting{
		{key: "SESSION_TOKEN_PEPPER", value: cfg.Session.TokenPepper},
		{key: "CSRF_TOKEN_PEPPER", value: cfg.Session.CSRFTokenPepper},
		{key: "TOTP_ENCRYPTION_KEY_B64", value: cfg.Session.TOTPEncryptionKey},
		{
			key:   "AUTH_ATTEMPT_FINGERPRINT_KEY",
			value: cfg.Authentication.AttemptFingerprintKey,
		},
		{key: "DB_PASSWORD", value: cfg.Database.Password},
	}

	for _, secret := range secrets {
		normalized := strings.TrimSpace(secret.value)
		if normalized == "" {
			return missingKeyError(secret.key)
		}
		if strings.EqualFold(normalized, "CHANGE_ME") {
			return invalidKeyError(secret.key, "must not be a placeholder")
		}
		if !allowsBundledExampleSecrets(cfg.App.Environment) &&
			isBundledExampleSecret(secret.key, normalized) {
			return invalidKeyError(
				secret.key,
				"must not use a bundled example value outside development or test",
			)
		}
	}

	return nil
}

func allowsBundledExampleSecrets(environment Environment) bool {
	switch strings.ToLower(strings.TrimSpace(string(environment))) {
	case "development", "test":
		return true
	default:
		return false
	}
}

func isBundledExampleSecret(key, value string) bool {
	if key != "TOTP_ENCRYPTION_KEY_B64" {
		return strings.HasPrefix(value, "development-only-")
	}

	decoded, err := base64.StdEncoding.DecodeString(value)
	if err != nil || len(decoded) != 32 {
		return false
	}
	for _, valueByte := range decoded {
		if valueByte != 0x11 {
			return false
		}
	}
	return true
}
