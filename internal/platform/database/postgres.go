package database

import (
	"context"
	"net"
	"net/url"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/korwlik23/go-api-starter/internal/platform/config"
	postgresgorm "gorm.io/driver/postgres"
)

func newPostgresConnection(
	cfg config.DatabaseConfig,
) (connection, error) {
	driverConfig, err := newPostgresDriverConfig(cfg)
	if err != nil {
		return connection{}, err
	}
	location, err := time.LoadLocation(cfg.TimeZone)
	if err != nil {
		return connection{}, err
	}

	pool := stdlib.OpenDB(
		*driverConfig,
		stdlib.OptionAfterConnect(func(_ context.Context, conn *pgx.Conn) error {
			registerPostgresTimestampCodec(conn.TypeMap(), location)
			return nil
		}),
	)
	return connection{
		pool: pool,
		dialector: postgresgorm.New(postgresgorm.Config{
			Conn: pool,
		}),
	}, nil
}

func newPostgresDriverConfig(
	cfg config.DatabaseConfig,
) (*pgx.ConnConfig, error) {
	if err := config.ValidateDatabase(cfg); err != nil {
		return nil, err
	}

	query := url.Values{}
	query.Set("sslmode", cfg.TLSMode)
	query.Set("timezone", cfg.TimeZone)
	connectionURL := url.URL{
		Scheme:   "postgres",
		User:     url.User(cfg.User),
		Host:     net.JoinHostPort(cfg.Host, strconv.Itoa(cfg.Port)),
		Path:     cfg.Name,
		RawQuery: query.Encode(),
	}
	driverConfig, err := pgx.ParseConfig(connectionURL.String())
	if err != nil {
		return nil, err
	}
	driverConfig.Password = cfg.Password
	driverConfig.ConnectTimeout = cfg.ConnectTimeout
	return driverConfig, nil
}

func registerPostgresTimestampCodec(typeMap *pgtype.Map, location *time.Location) {
	typeMap.RegisterType(&pgtype.Type{
		Name:  "timestamp",
		OID:   pgtype.TimestampOID,
		Codec: &pgtype.TimestampCodec{ScanLocation: location},
	})
}
