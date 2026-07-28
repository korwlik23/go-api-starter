package config

func validateDatabase(cfg Config) error {
	switch {
	case cfg.Database.Driver == "postgres" && cfg.Database.Flavor == "":
		return nil
	case cfg.Database.Driver == "mysql" && cfg.Database.Flavor == "mariadb":
		return nil
	case cfg.Database.Driver == "mysql" && cfg.Database.Flavor == "mysql":
		return nil
	default:
		return invalidKeyError("DB_DRIVER/DB_FLAVOR", "combination is unsupported")
	}
}
