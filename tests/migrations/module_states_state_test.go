package migrations_test

import (
	"context"
	"database/sql"
	"testing"

	"github.com/korwlik23/go-api-starter/internal/platform/config"
	"github.com/stretchr/testify/require"
)

func m18AssertModuleStateState(t *testing.T, ctx context.Context, db *sql.DB, cfg config.DatabaseConfig, offset byte) {
	t.Helper()
	state := m18NewModuleState("state_module", false, 120+offset)
	m18InsertModuleState(t, ctx, db, cfg, state)
	_, err := db.ExecContext(ctx,
		"UPDATE module_states SET enabled = "+migrationPlaceholder(cfg, 1)+", catalog_epoch = "+migrationPlaceholder(cfg, 2)+", reconcile_revision = "+migrationPlaceholder(cfg, 3)+" WHERE module_id = "+migrationPlaceholder(cfg, 4),
		true, 2, 2, state.moduleID,
	)
	require.NoError(t, err, "the future reconcile transaction may persist an explicit positive CAS transition")
	for _, column := range []string{"catalog_epoch", "reconcile_revision"} {
		_, err := db.ExecContext(ctx,
			"UPDATE module_states SET "+column+" = "+migrationPlaceholder(cfg, 1)+" WHERE module_id = "+migrationPlaceholder(cfg, 2),
			0, state.moduleID,
		)
		require.Error(t, err, "%s must remain positive", column)
	}
	var enabled any
	var epoch, revision int64
	require.NoError(t, db.QueryRowContext(ctx,
		"SELECT enabled, catalog_epoch, reconcile_revision FROM module_states WHERE module_id = "+migrationPlaceholder(cfg, 1), state.moduleID,
	).Scan(&enabled, &epoch, &revision))
	require.Equal(t, int64(1), m18BooleanValue(cfg, enabled))
	require.Equal(t, int64(2), epoch)
	require.Equal(t, int64(2), revision)
}
