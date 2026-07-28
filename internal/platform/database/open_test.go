package database

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/korwlik23/go-api-starter/internal/platform/config"
	"github.com/stretchr/testify/require"
	mysqlgorm "gorm.io/driver/mysql"
	postgresgorm "gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func TestD1SelectsDatabaseProfileAndConfiguresPool(t *testing.T) {
	tests := []struct {
		name              string
		driver            string
		flavor            string
		expectedDialector string
		expectedFactory   string
	}{
		{
			name:              "PostgreSQL",
			driver:            "postgres",
			expectedDialector: "postgres",
			expectedFactory:   "postgres",
		},
		{
			name:              "MariaDB",
			driver:            "mysql",
			flavor:            "mariadb",
			expectedDialector: "mysql",
			expectedFactory:   "mysql",
		},
		{
			name:              "Oracle MySQL compatibility profile",
			driver:            "mysql",
			flavor:            "mysql",
			expectedDialector: "mysql",
			expectedFactory:   "mysql",
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			cfg := d1DatabaseConfig(test.driver, test.flavor)
			pool := &recordingPool{}
			calledFactory := ""
			factories := factorySet{
				postgres: func(config.DatabaseConfig) (connection, error) {
					calledFactory = "postgres"
					return connection{
						pool:      pool,
						dialector: postgresgorm.New(postgresgorm.Config{Conn: pool}),
					}, nil
				},
				mysql: func(config.DatabaseConfig) (connection, error) {
					calledFactory = "mysql"
					return connection{
						pool: pool,
						dialector: mysqlgorm.New(mysqlgorm.Config{
							Conn:                      pool,
							SkipInitializeWithVersion: true,
						}),
					}, nil
				},
			}

			db, err := openWithFactories(context.Background(), cfg, factories)

			require.NoError(t, err)
			require.Equal(t, test.expectedFactory, calledFactory)
			require.Equal(t, test.expectedDialector, db.Dialector.Name())
			require.Equal(t, cfg.MaxOpenConnections, pool.maxOpenConnections)
			require.Equal(t, cfg.MaxIdleConnections, pool.maxIdleConnections)
			require.Equal(t, cfg.ConnectionMaxLifetime, pool.connectionMaxLifetime)
			require.Equal(t, 1, pool.pingCalls)
			require.Equal(t, 0, pool.closeCalls)
		})
	}
}

func TestD1RejectsUnsupportedProfileBeforeCallingFactory(t *testing.T) {
	called := false
	factory := func(config.DatabaseConfig) (connection, error) {
		called = true
		return connection{}, nil
	}
	cfg := d1DatabaseConfig("sqlite", "")

	db, err := openWithFactories(
		context.Background(),
		cfg,
		factorySet{postgres: factory, mysql: factory},
	)

	require.Nil(t, db)
	require.ErrorIs(t, err, ErrUnsupportedProfile)
	require.False(t, called)
}

func TestD1PublicOpenRejectsUnsupportedProfile(t *testing.T) {
	db, err := Open(
		context.Background(),
		d1DatabaseConfig("postgres", "mariadb"),
	)

	require.Nil(t, db)
	require.ErrorIs(t, err, ErrUnsupportedProfile)
}

func TestD1ClosesPoolAndRedactsPingFailure(t *testing.T) {
	const privateMarker = "private-driver-error-marker"
	pool := &recordingPool{pingError: errors.New(privateMarker)}
	cfg := d1DatabaseConfig("postgres", "")
	factories := factorySet{
		postgres: func(config.DatabaseConfig) (connection, error) {
			return connection{
				pool:      pool,
				dialector: postgresgorm.New(postgresgorm.Config{Conn: pool}),
			}, nil
		},
	}

	db, err := openWithFactories(context.Background(), cfg, factories)

	require.Nil(t, db)
	require.ErrorIs(t, err, ErrUnavailable)
	require.NotContains(t, err.Error(), privateMarker)
	require.Equal(t, 1, pool.closeCalls)
}

func TestD1ClosesPartialConnectionAndRedactsFactoryFailure(t *testing.T) {
	const privateMarker = "private-factory-error-marker"
	pool := &recordingPool{}
	cfg := d1DatabaseConfig("postgres", "")
	factories := factorySet{
		postgres: func(config.DatabaseConfig) (connection, error) {
			return connection{pool: pool}, errors.New(privateMarker)
		},
	}

	db, err := openWithFactories(context.Background(), cfg, factories)

	require.Nil(t, db)
	require.ErrorIs(t, err, ErrInvalidConfiguration)
	require.NotContains(t, err.Error(), privateMarker)
	require.Equal(t, 1, pool.closeCalls)
}

func TestD1ClosesPoolWhenContextIsCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	pool := &recordingPool{pingError: context.Canceled}
	cfg := d1DatabaseConfig("postgres", "")
	factories := factorySet{
		postgres: func(config.DatabaseConfig) (connection, error) {
			return connection{
				pool:      pool,
				dialector: postgresgorm.New(postgresgorm.Config{Conn: pool}),
			}, nil
		},
	}

	db, err := openWithFactories(ctx, cfg, factories)

	require.Nil(t, db)
	require.ErrorIs(t, err, context.Canceled)
	require.Equal(t, 1, pool.closeCalls)
}

func TestD1DefaultFactoriesProduceExpectedDialectors(t *testing.T) {
	tests := []struct {
		name              string
		cfg               config.DatabaseConfig
		factory           connectionFactory
		expectedDialector string
	}{
		{
			name:              "PostgreSQL",
			cfg:               d1DatabaseConfig("postgres", ""),
			factory:           newPostgresConnection,
			expectedDialector: "postgres",
		},
		{
			name:              "MariaDB",
			cfg:               d1DatabaseConfig("mysql", "mariadb"),
			factory:           newMySQLConnection,
			expectedDialector: "mysql",
		},
		{
			name:              "Oracle MySQL compatibility profile",
			cfg:               d1DatabaseConfig("mysql", "mysql"),
			factory:           newMySQLConnection,
			expectedDialector: "mysql",
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			candidate, err := test.factory(test.cfg)

			require.NoError(t, err)
			require.NotNil(t, candidate.pool)
			require.Equal(t, test.expectedDialector, candidate.dialector.Name())
			require.NoError(t, candidate.pool.Close())
		})
	}
}

func d1DatabaseConfig(driver string, flavor string) config.DatabaseConfig {
	port := 5432
	if driver == "mysql" {
		port = 3306
	}
	return config.DatabaseConfig{
		Driver:                driver,
		Flavor:                flavor,
		Host:                  "database.internal",
		Port:                  port,
		Name:                  "go_lang_starter_test_database",
		User:                  "go_lang_starter_test",
		Password:              "database-password",
		TLSMode:               "disable",
		TimeZone:              "UTC",
		ConnectTimeout:        5 * time.Second,
		MaxOpenConnections:    11,
		MaxIdleConnections:    5,
		ConnectionMaxLifetime: 5 * time.Minute,
	}
}

type recordingPool struct {
	maxOpenConnections    int
	maxIdleConnections    int
	connectionMaxLifetime time.Duration
	pingError             error
	pingFunc              func(context.Context) error
	pingCalls             int
	closeCalls            int
	pingDeadline          time.Time
	pingHasDeadline       bool
}

func (pool *recordingPool) SetMaxOpenConns(value int) {
	pool.maxOpenConnections = value
}

func (pool *recordingPool) SetMaxIdleConns(value int) {
	pool.maxIdleConnections = value
}

func (pool *recordingPool) SetConnMaxLifetime(value time.Duration) {
	pool.connectionMaxLifetime = value
}

func (pool *recordingPool) PingContext(ctx context.Context) error {
	pool.pingCalls++
	pool.pingDeadline, pool.pingHasDeadline = ctx.Deadline()
	if pool.pingFunc != nil {
		return pool.pingFunc(ctx)
	}
	return pool.pingError
}

func (pool *recordingPool) Close() error {
	pool.closeCalls++
	return nil
}

func (pool *recordingPool) PrepareContext(
	context.Context,
	string,
) (*sql.Stmt, error) {
	return nil, errors.New("unexpected PrepareContext call")
}

func (pool *recordingPool) ExecContext(
	context.Context,
	string,
	...any,
) (sql.Result, error) {
	return nil, errors.New("unexpected ExecContext call")
}

func (pool *recordingPool) QueryContext(
	context.Context,
	string,
	...any,
) (*sql.Rows, error) {
	return nil, errors.New("unexpected QueryContext call")
}

func (pool *recordingPool) QueryRowContext(
	context.Context,
	string,
	...any,
) *sql.Row {
	return &sql.Row{}
}

var _ gorm.ConnPool = (*recordingPool)(nil)
