package config

import (
	"bytes"
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
		{key: "TOTP_ENCRYPTION_KEYS_B64", value: cfg.Session.TOTPEncryptionKeys},
		{
			key:   "AUTH_ATTEMPT_FINGERPRINT_KEY",
			value: cfg.Authentication.AttemptFingerprintKey,
		},
		{
			key:   "RECOVERY_TOKEN_HMAC_KEYS_B64",
			value: cfg.Authentication.Recovery.TokenHMACKeys,
		},
		{
			key:   "RECOVERY_DELIVERY_ENCRYPTION_KEYS_B64",
			value: cfg.Authentication.Recovery.DeliveryEncryptionKeys,
		},
		{
			key:   "MFA_RECOVERY_CODE_HMAC_KEYS_B64",
			value: cfg.Authentication.Recovery.CodeHMACKeys,
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
	switch key {
	case "TOTP_ENCRYPTION_KEYS_B64":
		example := base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{0x11}, 32))
		return strings.Contains(value, example)
	case "RECOVERY_TOKEN_HMAC_KEYS_B64":
		example := base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{0x22}, 32))
		return strings.Contains(value, example)
	case "RECOVERY_DELIVERY_ENCRYPTION_KEYS_B64":
		example := base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{0x33}, 32))
		return strings.Contains(value, example)
	case "MFA_RECOVERY_CODE_HMAC_KEYS_B64":
		example := base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{0x44}, 32))
		return strings.Contains(value, example)
	default:
		return strings.HasPrefix(value, "development-only-")
	}
}
