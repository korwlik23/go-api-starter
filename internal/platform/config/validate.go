package config

import "bytes"

// Validate applies configuration policies after values have been loaded and
// converted to their runtime types.
func Validate(cfg Config) error {
	if err := validateSecrets(cfg); err != nil {
		return err
	}
	totpKeys, err := ParseTOTPKeyring(
		cfg.Session.TOTPActiveKeyVersion,
		cfg.Session.TOTPEncryptionKeys,
	)
	if err != nil {
		return err
	}
	tokenKeys, err := ParseRecoveryTokenKeyring(
		cfg.Authentication.Recovery.TokenActiveKeyVersion,
		cfg.Authentication.Recovery.TokenHMACKeys,
	)
	if err != nil {
		return err
	}
	deliveryKeys, err := ParseRecoveryDeliveryKeyring(
		cfg.Authentication.Recovery.DeliveryActiveKeyVersion,
		cfg.Authentication.Recovery.DeliveryEncryptionKeys,
	)
	if err != nil {
		return err
	}
	_, tokenKey := tokenKeys.Active()
	_, deliveryKey := deliveryKeys.Active()
	if bytes.Equal(tokenKey, deliveryKey) {
		return invalidKeyError(
			"recovery token HMAC and delivery encryption keys",
			"must be distinct",
		)
	}
	_, totpKey := totpKeys.Active()
	if bytes.Equal(totpKey, tokenKey) ||
		bytes.Equal(totpKey, deliveryKey) {
		return invalidKeyError(
			"recovery keyrings",
			"must be distinct from the TOTP encryption keyring",
		)
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
	_, err = ParseTrustedProxyCIDRs(cfg.HTTP.TrustedProxyCIDRs)
	return err
}
