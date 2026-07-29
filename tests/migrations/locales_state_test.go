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

func m14AssertLocaleStateConstraints(
	t *testing.T,
	ctx context.Context,
	db *sql.DB,
	cfg config.DatabaseConfig,
	fixtures m14Fixtures,
) {
	t.Helper()

	m14AssertBooleanStateConstraints(t, ctx, db, cfg, fixtures)
	m14AssertDefaultSlotConstraints(t, ctx, db, cfg, fixtures)
	m14AssertVersionAndTimeConstraints(t, ctx, db, cfg, fixtures.serbian)
}

func m14AssertBooleanStateConstraints(
	t *testing.T,
	ctx context.Context,
	db *sql.DB,
	cfg config.DatabaseConfig,
	fixtures m14Fixtures,
) {
	t.Helper()

	_, err := db.ExecContext(
		ctx,
		"UPDATE locales SET enabled = "+migrationPlaceholder(cfg, 1)+
			", user_selectable = "+migrationPlaceholder(cfg, 2)+
			" WHERE id = "+migrationPlaceholder(cfg, 3),
		true,
		true,
		fixtures.thai,
	)
	require.NoError(t, err)
	_, err = db.ExecContext(
		ctx,
		"UPDATE locales SET user_selectable = "+migrationPlaceholder(cfg, 1)+
			" WHERE id = "+migrationPlaceholder(cfg, 2),
		true,
		fixtures.chinese,
	)
	require.Error(t, err, "a disabled locale must not become user-selectable")
	m14RequireLocaleFlags(t, ctx, db, cfg, fixtures.chinese, false, false)

	for index, flags := range [][2]int{{2, 0}, {-1, 0}, {1, 2}, {1, -1}} {
		localeID := m14LocaleID(t, cfg, byte(70+index))
		_, err = db.ExecContext(
			ctx,
			"INSERT INTO locales (id, tag, name, direction, enabled, user_selectable) VALUES ("+
				migrationPlaceholders(cfg, 6)+")",
			localeID,
			fmt.Sprintf("zz-boolean-%d", index),
			fmt.Sprintf("Invalid boolean %d", index),
			"ltr",
			flags[0],
			flags[1],
		)
		require.Error(t, err, "boolean state %d must be rejected", index)
		m14RequireLocaleIDCount(t, ctx, db, cfg, localeID, 0)
	}
}

func m14AssertDefaultSlotConstraints(
	t *testing.T,
	ctx context.Context,
	db *sql.DB,
	cfg config.DatabaseConfig,
	fixtures m14Fixtures,
) {
	t.Helper()

	_, err := db.ExecContext(
		ctx,
		"UPDATE locales SET default_slot = 1 WHERE id = "+migrationPlaceholder(cfg, 1),
		fixtures.private,
	)
	require.Error(t, err, "a disabled locale must not become default")
	_, err = db.ExecContext(
		ctx,
		"UPDATE locales SET enabled = "+migrationPlaceholder(cfg, 1)+
			" WHERE id = "+migrationPlaceholder(cfg, 2),
		true,
		fixtures.english,
	)
	require.NoError(t, err)
	for _, invalidSlot := range []int{0, -1, 2} {
		_, err = db.ExecContext(
			ctx,
			"UPDATE locales SET default_slot = "+migrationPlaceholder(cfg, 1)+
				" WHERE id = "+migrationPlaceholder(cfg, 2),
			invalidSlot,
			fixtures.english,
		)
		require.Error(t, err, "invalid singleton default slot %d must be rejected", invalidSlot)
	}

	_, err = db.ExecContext(
		ctx,
		"UPDATE locales SET default_slot = 1 WHERE id = "+migrationPlaceholder(cfg, 1),
		fixtures.thai,
	)
	require.Error(t, err, "the default must be the root of its UI fallback chain")
	_, err = db.ExecContext(
		ctx,
		"UPDATE locales SET default_slot = 1 WHERE id = "+migrationPlaceholder(cfg, 1),
		fixtures.english,
	)
	require.NoError(t, err)
	_, err = db.ExecContext(
		ctx,
		"UPDATE locales SET enabled = "+migrationPlaceholder(cfg, 1)+
			" WHERE id = "+migrationPlaceholder(cfg, 2),
		false,
		fixtures.english,
	)
	require.Error(t, err, "the current default must remain enabled")

	secondDefaultID := m14LocaleID(t, cfg, 75)
	_, err = db.ExecContext(
		ctx,
		"INSERT INTO locales (id, tag, name, direction, enabled, default_slot) VALUES ("+
			migrationPlaceholders(cfg, 6)+")",
		secondDefaultID,
		"zz-second-default",
		"Second default",
		"ltr",
		true,
		1,
	)
	require.Error(t, err, "a second default insert must be rejected")
	m14RequireLocaleIDCount(t, ctx, db, cfg, secondDefaultID, 0)
	_, err = db.ExecContext(
		ctx,
		"UPDATE locales SET default_slot = 1 WHERE id = "+migrationPlaceholder(cfg, 1),
		fixtures.thai,
	)
	require.Error(t, err, "a second default update must be rejected")
	m14RequireDefaultLocale(t, ctx, db, cfg, fixtures.english)

	_, err = db.ExecContext(
		ctx,
		"UPDATE locales SET default_slot = NULL WHERE id = "+migrationPlaceholder(cfg, 1),
		fixtures.english,
	)
	require.NoError(t, err, "zero defaults must remain valid before initialization")
	m14RequireDefaultLocale(t, ctx, db, cfg, nil)
	_, err = db.ExecContext(
		ctx,
		"UPDATE locales SET default_slot = 1 WHERE id = "+migrationPlaceholder(cfg, 1),
		fixtures.english,
	)
	require.NoError(t, err)

	transaction, err := db.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer func() { _ = transaction.Rollback() }()
	_, err = transaction.ExecContext(
		ctx,
		"UPDATE locales SET default_slot = NULL WHERE id = "+migrationPlaceholder(cfg, 1),
		fixtures.english,
	)
	require.NoError(t, err)
	_, err = transaction.ExecContext(
		ctx,
		"UPDATE locales SET fallback_locale_id = NULL WHERE id = "+migrationPlaceholder(cfg, 1),
		fixtures.thai,
	)
	require.NoError(t, err)
	_, err = transaction.ExecContext(
		ctx,
		"UPDATE locales SET default_slot = 1 WHERE id = "+migrationPlaceholder(cfg, 1),
		fixtures.thai,
	)
	require.NoError(t, err)
	_, err = transaction.ExecContext(
		ctx,
		"UPDATE locales SET fallback_locale_id = "+migrationPlaceholder(cfg, 1)+
			" WHERE id = "+migrationPlaceholder(cfg, 2),
		fixtures.thai,
		fixtures.english,
	)
	require.NoError(t, err)
	require.NoError(t, transaction.Commit())
	m14RequireDefaultLocale(t, ctx, db, cfg, fixtures.thai)
	m14RequireFallback(t, ctx, db, cfg, fixtures.thai, nil)
	m14RequireFallback(t, ctx, db, cfg, fixtures.english, fixtures.thai)
}

func m14AssertVersionAndTimeConstraints(
	t *testing.T,
	ctx context.Context,
	db *sql.DB,
	cfg config.DatabaseConfig,
	controlID any,
) {
	t.Helper()

	createdAt := time.Now().UTC().Truncate(time.Microsecond)
	for index, version := range []int64{0, -1} {
		localeID := m14LocaleID(t, cfg, byte(80+index))
		_, err := db.ExecContext(
			ctx,
			"INSERT INTO locales (id, tag, name, direction, version) VALUES ("+
				migrationPlaceholders(cfg, 5)+")",
			localeID,
			fmt.Sprintf("zz-version-%d", index),
			fmt.Sprintf("Invalid version %d", index),
			"ltr",
			version,
		)
		require.Error(t, err)
		m14RequireLocaleIDCount(t, ctx, db, cfg, localeID, 0)
	}
	invalidTimeID := m14LocaleID(t, cfg, 82)
	_, err := db.ExecContext(
		ctx,
		"INSERT INTO locales (id, tag, name, direction, created_at, updated_at) VALUES ("+
			migrationPlaceholders(cfg, 6)+")",
		invalidTimeID,
		"zz-invalid-time",
		"Invalid time",
		"ltr",
		createdAt,
		createdAt.Add(-time.Second),
	)
	require.Error(t, err)
	m14RequireLocaleIDCount(t, ctx, db, cfg, invalidTimeID, 0)

	_, err = db.ExecContext(
		ctx,
		"UPDATE locales SET version = 0 WHERE id = "+migrationPlaceholder(cfg, 1),
		controlID,
	)
	require.Error(t, err)
	var version int64
	require.NoError(t, db.QueryRowContext(
		ctx,
		"SELECT version FROM locales WHERE id = "+migrationPlaceholder(cfg, 1),
		controlID,
	).Scan(&version))
	require.Equal(t, int64(1), version)
}

func m14RequireLocaleIDCount(
	t *testing.T,
	ctx context.Context,
	db *sql.DB,
	cfg config.DatabaseConfig,
	localeID any,
	expected int,
) {
	t.Helper()

	var count int
	require.NoError(t, db.QueryRowContext(
		ctx,
		"SELECT COUNT(*) FROM locales WHERE id = "+migrationPlaceholder(cfg, 1),
		localeID,
	).Scan(&count))
	require.Equal(t, expected, count)
}

func m14RequireLocaleFlags(
	t *testing.T,
	ctx context.Context,
	db *sql.DB,
	cfg config.DatabaseConfig,
	localeID any,
	expectedEnabled bool,
	expectedSelectable bool,
) {
	t.Helper()

	var enabled bool
	var selectable bool
	require.NoError(t, db.QueryRowContext(
		ctx,
		"SELECT enabled, user_selectable FROM locales WHERE id = "+migrationPlaceholder(cfg, 1),
		localeID,
	).Scan(&enabled, &selectable))
	require.Equal(t, expectedEnabled, enabled)
	require.Equal(t, expectedSelectable, selectable)
}

func m14RequireDefaultLocale(
	t *testing.T,
	ctx context.Context,
	db *sql.DB,
	cfg config.DatabaseConfig,
	expectedID any,
) {
	t.Helper()

	var count int
	require.NoError(t, db.QueryRowContext(
		ctx,
		"SELECT COUNT(*) FROM locales WHERE default_slot = 1",
	).Scan(&count))
	if expectedID == nil {
		require.Zero(t, count)
		return
	}
	require.Equal(t, 1, count)
	var expectedCount int
	require.NoError(t, db.QueryRowContext(
		ctx,
		"SELECT COUNT(*) FROM locales WHERE id = "+migrationPlaceholder(cfg, 1)+
			" AND default_slot = 1",
		expectedID,
	).Scan(&expectedCount))
	require.Equal(t, 1, expectedCount)
}
