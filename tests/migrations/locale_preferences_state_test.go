package migrations_test

import (
	"context"
	"database/sql"
	"fmt"
	"testing"
	"time"

	"github.com/korwlik23/go-api-starter/internal/platform/config"
	"github.com/stretchr/testify/require"
)

func m16AssertPreferenceState(
	t *testing.T,
	ctx context.Context,
	db *sql.DB,
	cfg config.DatabaseConfig,
	seedOffset byte,
) {
	t.Helper()

	userID := m3UserID(t, cfg, 50+seedOffset)
	m16InsertUser(t, ctx, db, cfg, userID, fmt.Sprintf("m16-cas-%d@example.test", seedOffset))
	m16InsertPreference(t, ctx, db, cfg, userID, true)
	initial := m16ReadPreference(t, ctx, db, cfg, userID)
	require.True(t, initial.allLanguages)
	require.Equal(t, int64(1), initial.version)

	subsetTime := initial.updatedAt.Add(time.Microsecond)
	require.Equal(t, int64(1), m16CASPreferenceState(
		t, ctx, db, cfg, userID, false, 1, subsetTime,
	))
	subset := m16ReadPreference(t, ctx, db, cfg, userID)
	require.False(t, subset.allLanguages)
	require.Equal(t, int64(2), subset.version)
	require.WithinDuration(t, subsetTime, subset.updatedAt, time.Microsecond)

	staleTime := subsetTime.Add(time.Microsecond)
	require.Zero(t, m16CASPreferenceState(
		t, ctx, db, cfg, userID, true, 1, staleTime,
	))
	require.Equal(t, subset, m16ReadPreference(t, ctx, db, cfg, userID))

	allTime := staleTime.Add(time.Microsecond)
	require.Equal(t, int64(1), m16CASPreferenceState(
		t, ctx, db, cfg, userID, true, 2, allTime,
	))
	allAgain := m16ReadPreference(t, ctx, db, cfg, userID)
	require.True(t, allAgain.allLanguages)
	require.Equal(t, int64(3), allAgain.version, "all -> subset -> all must preserve monotonic ABA state")
	require.WithinDuration(t, allTime, allAgain.updatedAt, time.Microsecond)

	for _, staleVersion := range []int64{1, 2} {
		require.Zero(t, m16CASPreferenceState(
			t,
			ctx,
			db,
			cfg,
			userID,
			false,
			staleVersion,
			allTime.Add(time.Duration(staleVersion)*time.Microsecond),
		))
	}
	require.Equal(t, allAgain, m16ReadPreference(t, ctx, db, cfg, userID))
}

func m16CASPreferenceState(
	t *testing.T,
	ctx context.Context,
	db *sql.DB,
	cfg config.DatabaseConfig,
	userID any,
	nextAllLanguages bool,
	expectedVersion int64,
	updatedAt time.Time,
) int64 {
	t.Helper()

	result, err := db.ExecContext(
		ctx,
		"UPDATE user_locale_preferences SET all_languages = "+migrationPlaceholder(cfg, 1)+
			", version = version + 1, updated_at = "+migrationPlaceholder(cfg, 2)+
			" WHERE user_id = "+migrationPlaceholder(cfg, 3)+
			" AND version = "+migrationPlaceholder(cfg, 4),
		nextAllLanguages,
		updatedAt,
		userID,
		expectedVersion,
	)
	require.NoError(t, err)
	rowsAffected, err := result.RowsAffected()
	require.NoError(t, err)
	return rowsAffected
}
