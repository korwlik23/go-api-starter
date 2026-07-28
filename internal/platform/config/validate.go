package config

// Validate applies configuration policies after values have been loaded and
// converted to their runtime types.
func Validate(cfg Config) error {
	return validateSecrets(cfg)
}
