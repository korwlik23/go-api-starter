package database

import (
	"context"
	"net/url"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/korwlik23/go-api-starter/internal/platform/config"
	"github.com/stretchr/testify/require"
	mysqlgorm "gorm.io/driver/mysql"
	postgresgorm "gorm.io/driver/postgres"
)

const d2SpecialPassword = "p@ ss:/?#%&=+漢字"

func TestD2PostgresDriverConfigPreservesStructuredValues(t *testing.T) {
	t.Parallel()

	cfg := d2DatabaseConfig("postgres", "")
	cfg.Host = "2001:db8::1"
	cfg.Password = d2SpecialPassword
	cfg.TLSMode = "verify-full"
	cfg.TimeZone = "Asia/Bangkok"
	cfg.ConnectTimeout = 7 * time.Second

	driverConfig, err := newPostgresDriverConfig(cfg)

	require.NoError(t, err)
	require.Equal(t, cfg.Host, driverConfig.Host)
	require.Equal(t, uint16(cfg.Port), driverConfig.Port)
	require.Equal(t, cfg.Name, driverConfig.Database)
	require.Equal(t, cfg.User, driverConfig.User)
	require.Equal(t, cfg.Password, driverConfig.Password)
	connectionURL, err := url.Parse(driverConfig.ConnString())
	require.NoError(t, err)
	_, hasPassword := connectionURL.User.Password()
	require.False(t, hasPassword)
	require.Equal(t, cfg.ConnectTimeout, driverConfig.ConnectTimeout)
	require.Equal(t, cfg.TimeZone, driverConfig.RuntimeParams["timezone"])
	require.NotNil(t, driverConfig.TLSConfig)
	require.Equal(t, cfg.Host, driverConfig.TLSConfig.ServerName)
}

func TestD2MySQLDriverConfigPreservesStructuredValues(t *testing.T) {
	t.Parallel()

	cfg := d2DatabaseConfig("mysql", "mariadb")
	cfg.Host = "2001:db8::1"
	cfg.Password = d2SpecialPassword
	cfg.TLSMode = "verify-full"
	cfg.ConnectTimeout = 7 * time.Second

	driverConfig, err := newMySQLDriverConfig(cfg)

	require.NoError(t, err)
	require.Equal(t, cfg.User, driverConfig.User)
	require.Equal(t, cfg.Password, driverConfig.Passwd)
	require.Equal(t, "tcp", driverConfig.Net)
	require.Equal(t, "[2001:db8::1]:3306", driverConfig.Addr)
	require.Equal(t, cfg.Name, driverConfig.DBName)
	require.True(t, driverConfig.ParseTime)
	require.Equal(t, cfg.TimeZone, driverConfig.Loc.String())
	require.Equal(t, "'+00:00'", driverConfig.Params["time_zone"])
	require.Equal(t, "'"+mysqlSessionSQLMode+"'", driverConfig.Params["sql_mode"])
	require.Equal(t, cfg.ConnectTimeout, driverConfig.Timeout)
	require.Equal(t, "true", driverConfig.TLSConfig)
	require.False(t, driverConfig.AllowFallbackToPlaintext)
}

func TestD2MySQLTLSModeMapping(t *testing.T) {
	t.Parallel()

	tests := []struct {
		mode        string
		expectedTLS string
	}{
		{mode: "disable", expectedTLS: ""},
		{mode: "require", expectedTLS: "skip-verify"},
		{mode: "verify-full", expectedTLS: "true"},
	}

	for _, test := range tests {
		test := test
		t.Run(test.mode, func(t *testing.T) {
			t.Parallel()

			cfg := d2DatabaseConfig("mysql", "mysql")
			cfg.TLSMode = test.mode

			driverConfig, err := newMySQLDriverConfig(cfg)

			require.NoError(t, err)
			require.Equal(t, test.expectedTLS, driverConfig.TLSConfig)
			require.False(t, driverConfig.AllowFallbackToPlaintext)
		})
	}
}

func TestD2GORMDialectorsDoNotRetainCredentialDSNs(t *testing.T) {
	t.Parallel()

	postgresConfig := d2DatabaseConfig("postgres", "")
	postgresConfig.Password = d2SpecialPassword
	postgresConnection, err := newPostgresConnection(postgresConfig)
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, postgresConnection.pool.Close())
	})

	postgresDialector, ok := postgresConnection.dialector.(*postgresgorm.Dialector)
	require.True(t, ok)
	require.Empty(t, postgresDialector.DSN)
	require.NotNil(t, postgresDialector.Conn)

	mysqlConfig := d2DatabaseConfig("mysql", "mariadb")
	mysqlConfig.Password = d2SpecialPassword
	mysqlConnection, err := newMySQLConnection(mysqlConfig)
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, mysqlConnection.pool.Close())
	})

	mysqlDialector, ok := mysqlConnection.dialector.(*mysqlgorm.Dialector)
	require.True(t, ok)
	require.Empty(t, mysqlDialector.DSN)
	require.Nil(t, mysqlDialector.DSNConfig)
	require.NotNil(t, mysqlDialector.Conn)
	require.True(t, mysqlDialector.SkipInitializeWithVersion)
	require.True(t, mysqlDialector.DontSupportRenameIndex)
	require.True(t, mysqlDialector.DontSupportRenameColumn)
	require.True(t, mysqlDialector.DontSupportForShareClause)
	require.True(t, mysqlDialector.DontSupportNullAsDefaultValue)
}

func TestD2OracleMySQLProfileKeepsModernCapabilityDefaults(t *testing.T) {
	t.Parallel()

	cfg := d2DatabaseConfig("mysql", "mysql")
	candidate, err := newMySQLConnection(cfg)
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, candidate.pool.Close())
	})

	dialector, ok := candidate.dialector.(*mysqlgorm.Dialector)
	require.True(t, ok)
	require.False(t, dialector.DontSupportRenameIndex)
	require.False(t, dialector.DontSupportRenameColumn)
	require.False(t, dialector.DontSupportForShareClause)
	require.False(t, dialector.DontSupportNullAsDefaultValue)
}

func TestD2PostgresTimestampCodecUsesConfiguredLocation(t *testing.T) {
	t.Parallel()

	location, err := time.LoadLocation("Asia/Bangkok")
	require.NoError(t, err)
	typeMap := pgtype.NewMap()

	registerPostgresTimestampCodec(typeMap, location)

	timestampType, ok := typeMap.TypeForOID(pgtype.TimestampOID)
	require.True(t, ok)
	codec, ok := timestampType.Codec.(*pgtype.TimestampCodec)
	require.True(t, ok)
	require.Same(t, location, codec.ScanLocation)
}

func TestD2OpenBoundsInitialPingWithConnectTimeout(t *testing.T) {
	t.Parallel()

	cfg := d2DatabaseConfig("postgres", "")
	cfg.ConnectTimeout = 2 * time.Second
	pool := &recordingPool{}
	before := time.Now()

	db, err := openWithFactories(
		context.Background(),
		cfg,
		d2PostgresFactory(pool),
	)

	require.NoError(t, err)
	require.NotNil(t, db)
	require.True(t, pool.pingHasDeadline)
	require.WithinDuration(t, before.Add(cfg.ConnectTimeout), pool.pingDeadline, 250*time.Millisecond)
}

func TestD2OpenPreservesEarlierCallerDeadline(t *testing.T) {
	t.Parallel()

	cfg := d2DatabaseConfig("postgres", "")
	cfg.ConnectTimeout = 5 * time.Second
	parentDeadline := time.Now().Add(2 * time.Second)
	ctx, cancel := context.WithDeadline(context.Background(), parentDeadline)
	defer cancel()
	pool := &recordingPool{}

	db, err := openWithFactories(ctx, cfg, d2PostgresFactory(pool))

	require.NoError(t, err)
	require.NotNil(t, db)
	require.True(t, pool.pingHasDeadline)
	require.WithinDuration(t, parentDeadline, pool.pingDeadline, 50*time.Millisecond)
}

func TestD2OpenRejectsInvalidDatabaseConfigurationBeforeFactory(t *testing.T) {
	t.Parallel()

	cfg := d2DatabaseConfig("postgres", "")
	cfg.Port = 0
	called := false
	factory := func(config.DatabaseConfig) (connection, error) {
		called = true
		return connection{}, nil
	}

	db, err := openWithFactories(
		context.Background(),
		cfg,
		factorySet{postgres: factory},
	)

	require.Nil(t, db)
	require.ErrorIs(t, err, ErrInvalidConfiguration)
	require.False(t, called)
}

func TestD2InternalConnectTimeoutReturnsUnavailableAndClosesPool(t *testing.T) {
	t.Parallel()

	cfg := d2DatabaseConfig("postgres", "")
	cfg.ConnectTimeout = 20 * time.Millisecond
	pool := &recordingPool{
		pingFunc: func(ctx context.Context) error {
			<-ctx.Done()
			return ctx.Err()
		},
	}

	db, err := openWithFactories(
		context.Background(),
		cfg,
		d2PostgresFactory(pool),
	)

	require.Nil(t, db)
	require.ErrorIs(t, err, ErrUnavailable)
	require.Equal(t, 1, pool.closeCalls)
}

func TestD2CallerDeadlineIsPropagatedAndClosesPool(t *testing.T) {
	t.Parallel()

	cfg := d2DatabaseConfig("postgres", "")
	cfg.ConnectTimeout = 5 * time.Second
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	pool := &recordingPool{
		pingFunc: func(ctx context.Context) error {
			<-ctx.Done()
			return ctx.Err()
		},
	}

	db, err := openWithFactories(ctx, cfg, d2PostgresFactory(pool))

	require.Nil(t, db)
	require.ErrorIs(t, err, context.DeadlineExceeded)
	require.Equal(t, 1, pool.closeCalls)
}

func d2DatabaseConfig(driver, flavor string) config.DatabaseConfig {
	return d1DatabaseConfig(driver, flavor)
}

func d2PostgresFactory(pool connectionPool) factorySet {
	return factorySet{
		postgres: func(config.DatabaseConfig) (connection, error) {
			return connection{
				pool:      pool,
				dialector: postgresgorm.New(postgresgorm.Config{Conn: pool}),
			}, nil
		},
	}
}
