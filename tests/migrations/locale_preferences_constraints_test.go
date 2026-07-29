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

func m16AssertPreferenceConstraints(
	t *testing.T,
	ctx context.Context,
	db *sql.DB,
	cfg config.DatabaseConfig,
	seedOffset byte,
) {
	t.Helper()

	m16AssertPreferenceRequiredValues(t, ctx, db, cfg, seedOffset)
	m16AssertPreferenceBooleanState(t, ctx, db, cfg, seedOffset)
	m16AssertPreferenceVersionAndTimeConstraints(t, ctx, db, cfg, seedOffset)
	m16AssertMySQLNonStrictPreferenceBoolean(t, ctx, db, cfg, seedOffset)
}

func m16AssertPreferenceRequiredValues(
	t *testing.T,
	ctx context.Context,
	db *sql.DB,
	cfg config.DatabaseConfig,
	seedOffset byte,
) {
	t.Helper()

	missingBooleanUser := m3UserID(t, cfg, 30+seedOffset)
	nullBooleanUser := m3UserID(t, cfg, 31+seedOffset)
	m16InsertUser(t, ctx, db, cfg, missingBooleanUser, fmt.Sprintf("m16-required-a-%d@example.test", seedOffset))
	m16InsertUser(t, ctx, db, cfg, nullBooleanUser, fmt.Sprintf("m16-required-b-%d@example.test", seedOffset))

	_, err := db.ExecContext(
		ctx,
		"INSERT INTO user_locale_preferences (all_languages) VALUES ("+migrationPlaceholder(cfg, 1)+")",
		true,
	)
	require.Error(t, err, "user identity must be explicit")

	_, err = db.ExecContext(
		ctx,
		"INSERT INTO user_locale_preferences (user_id, all_languages) VALUES ("+
			migrationPlaceholders(cfg, 2)+")",
		nil,
		true,
	)
	require.Error(t, err, "user identity must not be null")

	_, err = db.ExecContext(
		ctx,
		"INSERT INTO user_locale_preferences (user_id) VALUES ("+migrationPlaceholder(cfg, 1)+")",
		missingBooleanUser,
	)
	require.Error(t, err, "all-languages intent must be explicit")
	require.Zero(t, m16PreferenceCount(t, ctx, db, cfg, missingBooleanUser))

	_, err = db.ExecContext(
		ctx,
		"INSERT INTO user_locale_preferences (user_id, all_languages) VALUES ("+
			migrationPlaceholders(cfg, 2)+")",
		nullBooleanUser,
		nil,
	)
	require.Error(t, err, "all-languages intent must not be null")
	require.Zero(t, m16PreferenceCount(t, ctx, db, cfg, nullBooleanUser))

	unknownUser := m3UserID(t, cfg, 32+seedOffset)
	_, err = db.ExecContext(ctx, m16PreferenceInsertSQL(cfg), unknownUser, true)
	require.Error(t, err, "a preference header must reference an existing user")
	require.Zero(t, m16PreferenceCount(t, ctx, db, cfg, unknownUser))
}

func m16AssertPreferenceBooleanState(
	t *testing.T,
	ctx context.Context,
	db *sql.DB,
	cfg config.DatabaseConfig,
	seedOffset byte,
) {
	t.Helper()

	for index, invalid := range []int64{2, -1} {
		userID := m3UserID(t, cfg, byte(35+index)+seedOffset)
		m16InsertUser(
			t,
			ctx,
			db,
			cfg,
			userID,
			fmt.Sprintf("m16-boolean-%d-%d@example.test", seedOffset, index),
		)
		_, err := db.ExecContext(ctx, m16PreferenceInsertSQL(cfg), userID, invalid)
		require.Error(t, err, "non-boolean state %d must be rejected", invalid)
		require.Zero(t, m16PreferenceCount(t, ctx, db, cfg, userID))
	}
}

func m16AssertPreferenceVersionAndTimeConstraints(
	t *testing.T,
	ctx context.Context,
	db *sql.DB,
	cfg config.DatabaseConfig,
	seedOffset byte,
) {
	t.Helper()

	for index, version := range []int64{0, -1} {
		userID := m3UserID(t, cfg, byte(40+index)+seedOffset)
		m16InsertUser(
			t,
			ctx,
			db,
			cfg,
			userID,
			fmt.Sprintf("m16-version-%d-%d@example.test", seedOffset, index),
		)
		_, err := db.ExecContext(
			ctx,
			"INSERT INTO user_locale_preferences (user_id, all_languages, version) VALUES ("+
				migrationPlaceholders(cfg, 3)+")",
			userID,
			false,
			version,
		)
		require.Error(t, err, "version %d must be rejected", version)
		require.Zero(t, m16PreferenceCount(t, ctx, db, cfg, userID))
	}

	timeUser := m3UserID(t, cfg, 42+seedOffset)
	m16InsertUser(t, ctx, db, cfg, timeUser, fmt.Sprintf("m16-time-%d@example.test", seedOffset))
	createdAt := time.Now().UTC().Truncate(time.Microsecond)
	_, err := db.ExecContext(
		ctx,
		"INSERT INTO user_locale_preferences "+
			"(user_id, all_languages, created_at, updated_at) VALUES ("+
			migrationPlaceholders(cfg, 4)+")",
		timeUser,
		true,
		createdAt,
		createdAt.Add(-time.Second),
	)
	require.Error(t, err, "updated_at must not precede created_at")
	require.Zero(t, m16PreferenceCount(t, ctx, db, cfg, timeUser))

	controlUser := m3UserID(t, cfg, 43+seedOffset)
	m16InsertUser(t, ctx, db, cfg, controlUser, fmt.Sprintf("m16-state-control-%d@example.test", seedOffset))
	_, err = db.ExecContext(
		ctx,
		"INSERT INTO user_locale_preferences "+
			"(user_id, all_languages, created_at, updated_at) VALUES ("+
			migrationPlaceholders(cfg, 4)+")",
		controlUser,
		false,
		createdAt,
		createdAt,
	)
	require.NoError(t, err)
	control := m16ReadPreference(t, ctx, db, cfg, controlUser)
	require.Equal(t, int64(1), control.version)
	require.WithinDuration(t, createdAt, control.createdAt, time.Microsecond)
	require.WithinDuration(t, createdAt, control.updatedAt, time.Microsecond)

	_, err = db.ExecContext(
		ctx,
		"UPDATE user_locale_preferences SET version = 0 WHERE user_id = "+migrationPlaceholder(cfg, 1),
		controlUser,
	)
	require.Error(t, err, "an existing preference version must remain positive")
	require.Equal(t, control, m16ReadPreference(t, ctx, db, cfg, controlUser))
}

func m16AssertMySQLNonStrictPreferenceBoolean(
	t *testing.T,
	ctx context.Context,
	db *sql.DB,
	cfg config.DatabaseConfig,
	seedOffset byte,
) {
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
		restoreContext, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		require.NoError(t, m9SetSQLMode(restoreContext, connection, previousSQLMode))
		require.NoError(t, connection.Close())
	}()

	for index, invalid := range []int64{256, -129} {
		userID := m3UserID(t, cfg, byte(45+index)+seedOffset)
		m16InsertUser(
			t,
			ctx,
			db,
			cfg,
			userID,
			fmt.Sprintf("m16-nonstrict-%d-%d@example.test", seedOffset, index),
		)
		_, err = connection.ExecContext(ctx, m16PreferenceInsertSQL(cfg), userID, invalid)
		require.Error(t, err, "non-strict mode must not truncate invalid boolean %d into a valid state", invalid)
		require.Zero(t, m16PreferenceCountOnQueryer(t, ctx, connection, cfg, userID))
	}
}

type m16Queryer interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

func m16PreferenceCountOnQueryer(
	t *testing.T,
	ctx context.Context,
	queryer m16Queryer,
	cfg config.DatabaseConfig,
	userID any,
) int {
	t.Helper()
	var count int
	require.NoError(t, queryer.QueryRowContext(
		ctx,
		"SELECT COUNT(*) FROM user_locale_preferences WHERE user_id = "+migrationPlaceholder(cfg, 1),
		userID,
	).Scan(&count))
	return count
}
