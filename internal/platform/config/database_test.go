package config

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestA4(t *testing.T) {
	t.Parallel()

	validTests := []struct {
		name   string
		driver string
		flavor string
	}{
		{name: "PostgreSQL", driver: "postgres", flavor: ""},
		{name: "MariaDB", driver: "mysql", flavor: "mariadb"},
		{name: "Oracle MySQL compatibility profile", driver: "mysql", flavor: "mysql"},
	}

	for _, test := range validTests {
		test := test
		t.Run("valid/"+test.name, func(t *testing.T) {
			t.Parallel()

			cfg := validSecurityConfig()
			cfg.Database.Driver = test.driver
			cfg.Database.Flavor = test.flavor

			require.NoError(t, Validate(cfg))
		})
	}

	invalidTests := []struct {
		name   string
		driver string
		flavor string
	}{
		{name: "PostgreSQL with MySQL flavor", driver: "postgres", flavor: "mariadb"},
		{name: "MySQL driver without flavor", driver: "mysql", flavor: ""},
		{name: "MariaDB used as driver", driver: "mariadb", flavor: "mariadb"},
		{name: "unregistered driver", driver: "sqlite", flavor: ""},
	}

	for _, test := range invalidTests {
		test := test
		t.Run("invalid/"+test.name, func(t *testing.T) {
			t.Parallel()

			cfg := validSecurityConfig()
			cfg.Database.Driver = test.driver
			cfg.Database.Flavor = test.flavor

			err := Validate(cfg)
			require.EqualError(t, err, "config: DB_DRIVER/DB_FLAVOR combination is unsupported")
		})
	}
}
