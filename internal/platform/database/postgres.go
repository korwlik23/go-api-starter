package database

import (
	"net"
	"net/url"
	"strconv"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/korwlik23/go-api-starter/internal/platform/config"
	postgresgorm "gorm.io/driver/postgres"
)

func newPostgresConnection(
	cfg config.DatabaseConfig,
) (connection, error) {
	query := url.Values{}
	if cfg.TLSMode != "" {
		query.Set("sslmode", cfg.TLSMode)
	}
	if cfg.TimeZone != "" {
		query.Set("timezone", cfg.TimeZone)
	}
	connectionURL := url.URL{
		Scheme:   "postgres",
		User:     url.UserPassword(cfg.User, cfg.Password),
		Host:     net.JoinHostPort(cfg.Host, strconv.Itoa(cfg.Port)),
		Path:     cfg.Name,
		RawQuery: query.Encode(),
	}
	driverConfig, err := pgx.ParseConfig(connectionURL.String())
	if err != nil {
		return connection{}, err
	}

	pool := stdlib.OpenDB(*driverConfig)
	return connection{
		pool: pool,
		dialector: postgresgorm.New(postgresgorm.Config{
			Conn: pool,
		}),
	}, nil
}
