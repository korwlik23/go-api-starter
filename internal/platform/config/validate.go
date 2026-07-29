package config

// Validate applies configuration policies after values have been loaded and
// converted to their runtime types.
func Validate(cfg Config) error {
	if err := validateSecrets(cfg); err != nil {
		return err
	}
	if _, err := ParseTOTPKeyring(
		cfg.Session.TOTPActiveKeyVersion,
		cfg.Session.TOTPEncryptionKeys,
	); err != nil {
		return err
	}
	if err := ValidateDatabase(cfg.Database); err != nil {
		return err
	}
	if err := validateRateLimits(cfg); err != nil {
		return err
	}
	if err := ValidateAuthAttemptConfig(cfg.Authentication.Attempts); err != nil {
		return err
	}
	_, err := ParseTrustedProxyCIDRs(cfg.HTTP.TrustedProxyCIDRs)
	return err
}
