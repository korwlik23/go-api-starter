package config

import (
	"testing"
	"time"

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

func TestD2DatabaseConfigurationInvariants(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		mutate        func(*DatabaseConfig)
		expectedError string
	}{
		{
			name: "missing host",
			mutate: func(cfg *DatabaseConfig) {
				cfg.Host = " "
			},
			expectedError: "config: DB_HOST is required",
		},
		{
			name: "port below range",
			mutate: func(cfg *DatabaseConfig) {
				cfg.Port = 0
			},
			expectedError: "config: DB_PORT must be between 1 and 65535",
		},
		{
			name: "host includes port",
			mutate: func(cfg *DatabaseConfig) {
				cfg.Host = "database.internal:5432"
			},
			expectedError: "config: DB_HOST must be a hostname or unbracketed IP address",
		},
		{
			name: "IPv6 host includes brackets",
			mutate: func(cfg *DatabaseConfig) {
				cfg.Host = "[2001:db8::1]"
			},
			expectedError: "config: DB_HOST must be a hostname or unbracketed IP address",
		},
		{
			name: "host includes whitespace",
			mutate: func(cfg *DatabaseConfig) {
				cfg.Host = "database internal"
			},
			expectedError: "config: DB_HOST must be a hostname or unbracketed IP address",
		},
		{
			name: "host contains multiple PostgreSQL targets",
			mutate: func(cfg *DatabaseConfig) {
				cfg.Host = "primary.internal,secondary.internal"
			},
			expectedError: "config: DB_HOST must be a hostname or unbracketed IP address",
		},
		{
			name: "port above range",
			mutate: func(cfg *DatabaseConfig) {
				cfg.Port = 65536
			},
			expectedError: "config: DB_PORT must be between 1 and 65535",
		},
		{
			name: "missing database name",
			mutate: func(cfg *DatabaseConfig) {
				cfg.Name = ""
			},
			expectedError: "config: DB_NAME is required",
		},
		{
			name: "missing database user",
			mutate: func(cfg *DatabaseConfig) {
				cfg.User = ""
			},
			expectedError: "config: DB_USER is required",
		},
		{
			name: "missing database password",
			mutate: func(cfg *DatabaseConfig) {
				cfg.Password = ""
			},
			expectedError: "config: DB_PASSWORD is required",
		},
		{
			name: "unsupported PostgreSQL TLS mode",
			mutate: func(cfg *DatabaseConfig) {
				cfg.TLSMode = "preferred"
			},
			expectedError: "config: DB_TLS_MODE is unsupported for DB_DRIVER",
		},
		{
			name: "unsupported MySQL TLS mode",
			mutate: func(cfg *DatabaseConfig) {
				cfg.Driver = "mysql"
				cfg.Flavor = "mariadb"
				cfg.Port = 3306
				cfg.TLSMode = "verify-ca"
			},
			expectedError: "config: DB_TLS_MODE is unsupported for DB_DRIVER",
		},
		{
			name: "invalid timezone",
			mutate: func(cfg *DatabaseConfig) {
				cfg.TimeZone = "Not/A_Time_Zone"
			},
			expectedError: "config: DB_TIME_ZONE must be valid",
		},
		{
			name: "MySQL non UTC timezone",
			mutate: func(cfg *DatabaseConfig) {
				cfg.Driver = "mysql"
				cfg.Flavor = "mariadb"
				cfg.Port = 3306
				cfg.TimeZone = "Asia/Bangkok"
			},
			expectedError: "config: DB_TIME_ZONE must be UTC for DB_DRIVER",
		},
		{
			name: "zero connect timeout",
			mutate: func(cfg *DatabaseConfig) {
				cfg.ConnectTimeout = 0
			},
			expectedError: "config: DB_CONNECT_TIMEOUT must be greater than zero",
		},
		{
			name: "zero maximum open connections",
			mutate: func(cfg *DatabaseConfig) {
				cfg.MaxOpenConnections = 0
			},
			expectedError: "config: DB_MAX_OPEN_CONNECTIONS must be greater than zero",
		},
		{
			name: "zero maximum idle connections",
			mutate: func(cfg *DatabaseConfig) {
				cfg.MaxIdleConnections = 0
			},
			expectedError: "config: DB_MAX_IDLE_CONNECTIONS must be greater than zero",
		},
		{
			name: "idle connections exceed open connections",
			mutate: func(cfg *DatabaseConfig) {
				cfg.MaxIdleConnections = cfg.MaxOpenConnections + 1
			},
			expectedError: "config: DB_MAX_IDLE_CONNECTIONS must not exceed DB_MAX_OPEN_CONNECTIONS",
		},
		{
			name: "zero connection lifetime",
			mutate: func(cfg *DatabaseConfig) {
				cfg.ConnectionMaxLifetime = 0
			},
			expectedError: "config: DB_CONNECTION_MAX_LIFETIME must be greater than zero",
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			cfg := validSecurityConfig().Database
			test.mutate(&cfg)

			err := ValidateDatabase(cfg)
			require.EqualError(t, err, test.expectedError)
			if cfg.Password != "" {
				require.NotContains(t, err.Error(), cfg.Password)
			}
		})
	}
}

func TestD2DatabaseTLSModes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		driver string
		flavor string
		port   int
		modes  []string
	}{
		{
			name:   "PostgreSQL",
			driver: "postgres",
			port:   5432,
			modes:  []string{"disable", "require", "verify-ca", "verify-full"},
		},
		{
			name:   "MariaDB",
			driver: "mysql",
			flavor: "mariadb",
			port:   3306,
			modes:  []string{"disable", "require", "verify-full"},
		},
		{
			name:   "Oracle MySQL compatibility profile",
			driver: "mysql",
			flavor: "mysql",
			port:   3306,
			modes:  []string{"disable", "require", "verify-full"},
		},
	}

	for _, test := range tests {
		test := test
		for _, mode := range test.modes {
			mode := mode
			t.Run(test.name+"/"+mode, func(t *testing.T) {
				t.Parallel()

				cfg := validSecurityConfig().Database
				cfg.Driver = test.driver
				cfg.Flavor = test.flavor
				cfg.Port = test.port
				cfg.TLSMode = mode
				if cfg.Driver == "postgres" {
					cfg.TimeZone = "Asia/Bangkok"
				}
				cfg.ConnectTimeout = 7 * time.Second

				require.NoError(t, ValidateDatabase(cfg))
			})
		}
	}
}
