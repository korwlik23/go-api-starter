package migrations_test

import (
	"bytes"
	"context"
	"database/sql"
	"fmt"
	"testing"
	"time"

	"github.com/korwlik23/go-api-starter/internal/platform/config"
	"github.com/stretchr/testify/require"
)

type m18ModuleState struct {
	moduleID          string
	enabled           any
	moduleChecksum    []byte
	configChecksum    []byte
	catalogChecksum   []byte
	catalogEpoch      int64
	reconcileRevision int64
}

func m18AssertModuleStateBehavior(t *testing.T, ctx context.Context, db *sql.DB, cfg config.DatabaseConfig, offset byte) {
	t.Helper()
	before := time.Now().UTC().Add(-time.Second)
	first := m18NewModuleState("accounts", true, 10+offset)
	second := m18NewModuleState("identity", false, 20+offset)
	m18InsertModuleState(t, ctx, db, cfg, first)
	m18InsertModuleState(t, ctx, db, cfg, second)
	after := time.Now().UTC().Add(time.Second)

	for _, expected := range []m18ModuleState{first, second} {
		var actual m18ModuleState
		var recordedAt time.Time
		require.NoError(t, db.QueryRowContext(ctx,
			"SELECT module_id, enabled, module_checksum, config_checksum, catalog_checksum, catalog_epoch, reconcile_revision, reconciled_at FROM module_states WHERE module_id = "+migrationPlaceholder(cfg, 1),
			expected.moduleID,
		).Scan(&actual.moduleID, &actual.enabled, &actual.moduleChecksum, &actual.configChecksum, &actual.catalogChecksum, &actual.catalogEpoch, &actual.reconcileRevision, &recordedAt))
		require.Equal(t, expected.moduleID, actual.moduleID)
		require.Equal(t, m18BooleanValue(cfg, expected.enabled), m18BooleanValue(cfg, actual.enabled))
		require.Equal(t, expected.moduleChecksum, actual.moduleChecksum)
		require.Equal(t, expected.configChecksum, actual.configChecksum)
		require.Equal(t, expected.catalogChecksum, actual.catalogChecksum)
		require.Equal(t, expected.catalogEpoch, actual.catalogEpoch)
		require.Equal(t, expected.reconcileRevision, actual.reconcileRevision)
		require.False(t, recordedAt.Before(before))
		require.False(t, recordedAt.After(after))
	}

	_, err := db.ExecContext(ctx, m18ModuleStateInsertSQL(cfg), m18ModuleStateArguments(first)...)
	require.Error(t, err, "module ID is the sole state identity")
}

func m18NewModuleState(moduleID string, enabled bool, seed byte) m18ModuleState {
	return m18ModuleState{
		moduleID:          moduleID,
		enabled:           enabled,
		moduleChecksum:    m18Checksum(seed),
		configChecksum:    m18Checksum(seed + 1),
		catalogChecksum:   m18Checksum(seed + 2),
		catalogEpoch:      1,
		reconcileRevision: 1,
	}
}

func m18Checksum(seed byte) []byte { return bytes.Repeat([]byte{seed}, 32) }

func m18InsertModuleState(t *testing.T, ctx context.Context, db *sql.DB, cfg config.DatabaseConfig, state m18ModuleState) {
	t.Helper()
	_, err := db.ExecContext(ctx, m18ModuleStateInsertSQL(cfg), m18ModuleStateArguments(state)...)
	require.NoError(t, err)
}

func m18ModuleStateInsertSQL(cfg config.DatabaseConfig) string {
	return "INSERT INTO module_states (module_id, enabled, module_checksum, config_checksum, catalog_checksum, catalog_epoch, reconcile_revision) VALUES (" + migrationPlaceholders(cfg, 7) + ")"
}

func m18ModuleStateArguments(state m18ModuleState) []any {
	return []any{state.moduleID, state.enabled, state.moduleChecksum, state.configChecksum, state.catalogChecksum, state.catalogEpoch, state.reconcileRevision}
}

func m18BooleanValue(cfg config.DatabaseConfig, value any) int64 {
	if cfg.Driver == "postgres" {
		if value.(bool) {
			return 1
		}
		return 0
	}
	switch converted := value.(type) {
	case bool:
		if converted {
			return 1
		}
		return 0
	case int64:
		return converted
	case []uint8:
		if string(converted) == "1" {
			return 1
		}
		return 0
	default:
		panic(fmt.Sprintf("unexpected boolean type %T", value))
	}
}
