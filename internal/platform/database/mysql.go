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
	location, err := time.LoadLocation(cfg.TimeZone)
	if err != nil {
		return connection{}, err
	}

	driverConfig := mysqldriver.NewConfig()
	driverConfig.User = cfg.User
	driverConfig.Passwd = cfg.Password
	driverConfig.Net = "tcp"
	driverConfig.Addr = net.JoinHostPort(cfg.Host, strconv.Itoa(cfg.Port))
	driverConfig.DBName = cfg.Name
	driverConfig.ParseTime = true
	driverConfig.Loc = location
	if cfg.TLSMode != "" && cfg.TLSMode != "disable" {
		driverConfig.TLSConfig = cfg.TLSMode
	}

	connector, err := mysqldriver.NewConnector(driverConfig)
	if err != nil {
		return connection{}, err
	}
	pool := sql.OpenDB(connector)
	return connection{
		pool: pool,
		dialector: mysqlgorm.New(mysqlgorm.Config{
			Conn:                      pool,
			SkipInitializeWithVersion: true,
		}),
	}, nil
}
