package database

import (
	"context"
	"fmt"
	"io/fs"
	"strconv"
	"testing"
	"testing/fstest"
	"time"

	"github.com/stretchr/testify/require"
)

func TestD3ProductionMigratorIntegration(t *testing.T) {
	if !databaseIntegrationEnabled() {
		t.Skip("database integration test is disabled")
	}

	cfg := databaseIntegrationConfig(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	db, err := Open(ctx, cfg)
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, sqlDB.Close())
	})

	token := strconv.FormatInt(time.Now().UnixNano(), 36)
	alphaModuleID := "d3_alpha_" + token
	betaModuleID := "d3_beta_" + token
	alphaTable := "d3_table_alpha_" + token
	betaTable := "d3_table_beta_" + token
	cleanupTables := []string{
		alphaTable,
		betaTable,
		migrationVersionTable(alphaModuleID),
		migrationVersionTable(betaModuleID),
	}
	t.Cleanup(func() {
		cleanupContext, cleanupCancel := context.WithTimeout(
			context.Background(),
			10*time.Second,
		)
		defer cleanupCancel()
		for _, table := range cleanupTables {
			_, cleanupError := sqlDB.ExecContext(
				cleanupContext,
				"DROP TABLE IF EXISTS "+table,
			)
			require.NoError(t, cleanupError)
		}
	})

	modules := []MigrationModule{
		d3IntegrationMigrationModule(
			betaModuleID,
			betaTable,
			alphaModuleID,
		),
		d3IntegrationMigrationModule(alphaModuleID, alphaTable),
	}
	migrator, err := NewMigrator(sqlDB, cfg, modules)
	require.NoError(t, err)

	require.NoError(t, migrator.Up(ctx))

	expectedProbeValue := d3ExpectedProbeValue(cfg.Driver, cfg.Flavor)
	for _, table := range []string{alphaTable, betaTable} {
		var probeValue int
		err = sqlDB.QueryRowContext(
			ctx,
			"SELECT id FROM "+table,
		).Scan(&probeValue)
		require.NoError(t, err)
		require.Equal(t, expectedProbeValue, probeValue)
	}
	for _, moduleID := range []string{alphaModuleID, betaModuleID} {
		var appliedCount int
		err = sqlDB.QueryRowContext(
			ctx,
			"SELECT COUNT(*) FROM "+
				migrationVersionTable(moduleID)+
				" WHERE version_id = 1",
		).Scan(&appliedCount)
		require.NoError(t, err)
		require.Equal(t, 1, appliedCount)
	}
}

func d3IntegrationMigrationModule(
	moduleID string,
	table string,
	dependencies ...string,
) MigrationModule {
	return MigrationModule{
		ID:           moduleID,
		Dependencies: dependencies,
		Bundles: map[MigrationDialect]fs.FS{
			MigrationDialectPostgres: d3IntegrationMigrationFS(table, 101),
			MigrationDialectMariaDB:  d3IntegrationMigrationFS(table, 202),
			MigrationDialectMySQL:    d3IntegrationMigrationFS(table, 303),
		},
	}
}

func d3IntegrationMigrationFS(table string, probeValue int) fstest.MapFS {
	sqlMigration := fmt.Sprintf(
		"-- +goose Up\n"+
			"CREATE TABLE %s (id BIGINT PRIMARY KEY);\n"+
			"INSERT INTO %s (id) VALUES (%d);\n"+
			"-- +goose Down\n"+
			"DROP TABLE %s;\n",
		table,
		table,
		probeValue,
		table,
	)
	return fstest.MapFS{
		"000001_create_probe.sql": {
			Data: []byte(sqlMigration),
		},
	}
}

func d3ExpectedProbeValue(driver, flavor string) int {
	switch {
	case driver == "postgres" && flavor == "":
		return 101
	case driver == "mysql" && flavor == "mariadb":
		return 202
	case driver == "mysql" && flavor == "mysql":
		return 303
	default:
		return 0
	}
}
