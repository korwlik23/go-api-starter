package config

import "strings"

type secretSetting struct {
	key   string
	value string
}

func validateSecrets(cfg Config) error {
	secrets := []secretSetting{
		{key: "SESSION_TOKEN_PEPPER", value: cfg.Session.TokenPepper},
		{key: "CSRF_TOKEN_PEPPER", value: cfg.Session.CSRFTokenPepper},
		{key: "TOTP_ENCRYPTION_KEY_B64", value: cfg.Session.TOTPEncryptionKey},
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
	}

	return nil
}
