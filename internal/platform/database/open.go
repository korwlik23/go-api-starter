package database

import (
	"context"
	"errors"
	"time"

	"github.com/korwlik23/go-api-starter/internal/platform/config"
	"gorm.io/gorm"
)

var (
	ErrUnsupportedProfile   = errors.New("database: unsupported driver/flavor")
	ErrInvalidConfiguration = errors.New("database: invalid configuration")
	ErrUnavailable          = errors.New("database: unavailable")
)

type connectionPool interface {
	gorm.ConnPool
	PingContext(context.Context) error
	Close() error
	SetMaxOpenConns(int)
	SetMaxIdleConns(int)
	SetConnMaxLifetime(time.Duration)
}

type connection struct {
	pool      connectionPool
	dialector gorm.Dialector
}

type connectionFactory func(config.DatabaseConfig) (connection, error)

type factorySet struct {
	postgres connectionFactory
	mysql    connectionFactory
}

var defaultFactories = factorySet{
	postgres: newPostgresConnection,
	mysql:    newMySQLConnection,
}

// Open creates and verifies the configured SQL connection pool.
func Open(
	ctx context.Context,
	cfg config.DatabaseConfig,
) (*gorm.DB, error) {
	return openWithFactories(ctx, cfg, defaultFactories)
}

func openWithFactories(
	ctx context.Context,
	cfg config.DatabaseConfig,
	factories factorySet,
) (*gorm.DB, error) {
	factory, err := selectFactory(cfg, factories)
	if err != nil {
		return nil, err
	}
	if err := config.ValidateDatabase(cfg); err != nil {
		return nil, ErrInvalidConfiguration
	}
	candidate, err := factory(cfg)
	if err != nil || candidate.pool == nil || candidate.dialector == nil {
		if candidate.pool != nil {
			_ = candidate.pool.Close()
		}
		return nil, ErrInvalidConfiguration
	}

	candidate.pool.SetMaxOpenConns(cfg.MaxOpenConnections)
	candidate.pool.SetMaxIdleConns(cfg.MaxIdleConnections)
	candidate.pool.SetConnMaxLifetime(cfg.ConnectionMaxLifetime)

	db, err := gorm.Open(
		candidate.dialector,
		&gorm.Config{DisableAutomaticPing: true},
	)
	if err != nil {
		_ = candidate.pool.Close()
		return nil, ErrUnavailable
	}
	pingContext, cancelPing := context.WithTimeout(ctx, cfg.ConnectTimeout)
	defer cancelPing()
	if err := candidate.pool.PingContext(pingContext); err != nil {
		_ = candidate.pool.Close()
		if ctxErr := ctx.Err(); ctxErr != nil {
			return nil, ctxErr
		}
		return nil, ErrUnavailable
	}
	return db, nil
}

func selectFactory(
	cfg config.DatabaseConfig,
	factories factorySet,
) (connectionFactory, error) {
	switch {
	case cfg.Driver == "postgres" && cfg.Flavor == "":
		if factories.postgres != nil {
			return factories.postgres, nil
		}
	case cfg.Driver == "mysql" &&
		(cfg.Flavor == "mariadb" || cfg.Flavor == "mysql"):
		if factories.mysql != nil {
			return factories.mysql, nil
		}
	}
	return nil, ErrUnsupportedProfile
}
