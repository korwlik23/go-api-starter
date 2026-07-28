package config

// Validate applies configuration policies after values have been loaded and
// converted to their runtime types.
func Validate(cfg Config) error {
	if err := validateSecrets(cfg); err != nil {
		return err
	}
	if err := validateDatabase(cfg); err != nil {
		return err
	}
	return validateRateLimits(cfg)
}
