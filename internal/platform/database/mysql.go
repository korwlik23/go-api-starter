package database

import (
	"database/sql"
	"net"
	"strconv"
	"time"

	mysqldriver "github.com/go-sql-driver/mysql"
	"github.com/korwlik23/go-api-starter/internal/platform/config"
	mysqlgorm "gorm.io/driver/mysql"
)

func newMySQLConnection(
	cfg config.DatabaseConfig,
) (connection, error) {
	driverConfig, err := newMySQLDriverConfig(cfg)
	if err != nil {
		return connection{}, err
	}

	connector, err := mysqldriver.NewConnector(driverConfig)
	if err != nil {
		return connection{}, err
	}
	pool := sql.OpenDB(connector)
	dialectorConfig := mysqlgorm.Config{
		Conn:                      pool,
		SkipInitializeWithVersion: true,
	}
	if cfg.Flavor == "mariadb" {
		dialectorConfig.DontSupportRenameIndex = true
		dialectorConfig.DontSupportRenameColumn = true
		dialectorConfig.DontSupportForShareClause = true
		dialectorConfig.DontSupportNullAsDefaultValue = true
	}
	return connection{
		pool:      pool,
		dialector: mysqlgorm.New(dialectorConfig),
	}, nil
}

func newMySQLDriverConfig(
	cfg config.DatabaseConfig,
) (*mysqldriver.Config, error) {
	if err := config.ValidateDatabase(cfg); err != nil {
		return nil, err
	}

	location, err := time.LoadLocation(cfg.TimeZone)
	if err != nil {
		return nil, err
	}

	driverConfig := mysqldriver.NewConfig()
	driverConfig.User = cfg.User
	driverConfig.Passwd = cfg.Password
	driverConfig.Net = "tcp"
	driverConfig.Addr = net.JoinHostPort(cfg.Host, strconv.Itoa(cfg.Port))
	driverConfig.DBName = cfg.Name
	driverConfig.ParseTime = true
	driverConfig.Loc = location
	driverConfig.Params = map[string]string{
		"time_zone": "'+00:00'",
	}
	driverConfig.Timeout = cfg.ConnectTimeout
	switch cfg.TLSMode {
	case "require":
		driverConfig.TLSConfig = "skip-verify"
	case "verify-full":
		driverConfig.TLSConfig = "true"
	}
	driverConfig.AllowFallbackToPlaintext = false
	return driverConfig, nil
}
