package migrations_test

import (
	"context"
	"database/sql"
	"strings"
	"testing"

	"github.com/korwlik23/go-api-starter/internal/platform/config"
	"github.com/stretchr/testify/require"
)

func m18AssertModuleStateConstraints(t *testing.T, ctx context.Context, db *sql.DB, cfg config.DatabaseConfig, offset byte) {
	t.Helper()
	m18AssertModuleIDConstraints(t, ctx, db, cfg, offset)
	m18AssertModuleChecksumConstraints(t, ctx, db, cfg, offset)
	m18AssertMySQLNonStrictModuleBounds(t, ctx, db, cfg, offset)
}

func m18AssertModuleIDConstraints(t *testing.T, ctx context.Context, db *sql.DB, cfg config.DatabaseConfig, offset byte) {
	t.Helper()
	maximumID := "a" + strings.Repeat("b", 39)
	maximum := m18NewModuleState(maximumID, true, 40+offset)
	m18InsertModuleState(t, ctx, db, cfg, maximum)
	for index, moduleID := range []string{"", "Accounts", "1accounts", "_accounts", "accounts-legacy", "accounts.module", "accountsไทย", "a" + strings.Repeat("b", 40)} {
		state := m18NewModuleState(moduleID, true, byte(41+index)+offset)
		_, err := db.ExecContext(ctx, m18ModuleStateInsertSQL(cfg), m18ModuleStateArguments(state)...)
		require.Error(t, err, "invalid module ID %d must fail", index)
	}
}

func m18AssertModuleChecksumConstraints(t *testing.T, ctx context.Context, db *sql.DB, cfg config.DatabaseConfig, offset byte) {
	t.Helper()
	base := m18NewModuleState("operations", true, 60+offset)
	for index, field := range []string{"enabled", "module_checksum", "config_checksum", "catalog_checksum", "catalog_epoch", "reconcile_revision"} {
		state := base
		state.moduleID = "constraint_" + string(rune('a'+index))
		switch field {
		case "enabled":
			state.enabled = nil
		case "module_checksum":
			state.moduleChecksum = nil
		case "config_checksum":
			state.configChecksum = nil
		case "catalog_checksum":
			state.catalogChecksum = nil
		case "catalog_epoch":
			state.catalogEpoch = 0
		case "reconcile_revision":
			state.reconcileRevision = 0
		}
		_, err := db.ExecContext(ctx, m18ModuleStateInsertSQL(cfg), m18ModuleStateArguments(state)...)
		require.Error(t, err, "%s must be explicit and valid", field)
	}
	for index, checksum := range [][]byte{m18Checksum(1)[:31], append(m18Checksum(2), 2)} {
		state := base
		state.moduleID = "checksum_" + string(rune('a'+index))
		state.moduleChecksum = checksum
		_, err := db.ExecContext(ctx, m18ModuleStateInsertSQL(cfg), m18ModuleStateArguments(state)...)
		require.Error(t, err, "checksum length %d must fail", len(checksum))
	}
	for index, enabled := range []any{int64(-1), int64(2)} {
		state := base
		state.moduleID = "enabled_" + string(rune('a'+index))
		state.enabled = enabled
		_, err := db.ExecContext(ctx, m18ModuleStateInsertSQL(cfg), m18ModuleStateArguments(state)...)
		require.Error(t, err, "invalid enabled state %d must fail", index)
	}
}

func m18AssertMySQLNonStrictModuleBounds(t *testing.T, ctx context.Context, db *sql.DB, cfg config.DatabaseConfig, offset byte) {
	t.Helper()
	if cfg.Driver != "mysql" {
		return
	}
	connection, err := db.Conn(ctx)
	require.NoError(t, err)
	var previousSQLMode string
	require.NoError(t, connection.QueryRowContext(ctx, "SELECT @@SESSION.sql_mode").Scan(&previousSQLMode))
	require.NoError(t, m9SetSQLMode(ctx, connection, ""))
	defer func() {
		require.NoError(t, m9SetSQLMode(context.Background(), connection, previousSQLMode))
		require.NoError(t, connection.Close())
	}()
	state := m18NewModuleState("a"+strings.Repeat("b", 40), true, 80+offset)
	_, err = connection.ExecContext(ctx, m18ModuleStateInsertSQL(cfg), m18ModuleStateArguments(state)...)
	require.Error(t, err, "non-strict mode must not truncate an overlong module ID into a valid ID")
}
