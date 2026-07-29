package migrations_test

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/korwlik23/go-api-starter/internal/platform/config"
	"github.com/stretchr/testify/require"
)

func m14AssertTagConstraints(
	t *testing.T,
	ctx context.Context,
	db *sql.DB,
	cfg config.DatabaseConfig,
	seedOffset byte,
) {
	t.Helper()

	baseCount := m14LocaleCount(t, ctx, db)
	maximumTag := "abc-" + strings.Repeat("abcdefgh-", 27) + "abcdefgh"
	require.Len(t, maximumTag, 255)
	maximumTagID := m14LocaleID(t, cfg, 39+seedOffset)
	m14InsertLocale(
		t,
		ctx,
		db,
		cfg,
		maximumTagID,
		maximumTag,
		"Maximum tag boundary",
		"ltr",
	)
	m14RequireLocaleIDCount(t, ctx, db, cfg, maximumTagID, 1)
	_, err := db.ExecContext(
		ctx,
		"DELETE FROM locales WHERE id = "+migrationPlaceholder(cfg, 1),
		maximumTagID,
	)
	require.NoError(t, err)

	invalidTags := []string{
		"",
		"FR",
		" fr",
		"fr ",
		"1",
		"123",
		"a",
		"x",
		"i",
		"-en",
		"en-",
		"en--us",
		"en_us",
		"en/us",
		"en\\us",
		"en.us",
		"en%us",
		"en\nus",
		"еn",
		"en-abcdefghi",
		strings.Repeat("abcdefgh-", 28) + "abcd",
	}
	for index, tag := range invalidTags {
		localeID := m14LocaleID(t, cfg, byte(20+index)+seedOffset)
		_, err = db.ExecContext(
			ctx,
			m14LocaleInsertSQL(cfg),
			localeID,
			tag,
			fmt.Sprintf("Invalid tag %d", index),
			"ltr",
		)
		require.Error(t, err, "invalid locale tag %d must be rejected", index)
		m14RequireLocaleIDCount(t, ctx, db, cfg, localeID, 0)
	}
	require.Equal(t, baseCount, m14LocaleCount(t, ctx, db))
}

func m14AssertNameConstraints(
	t *testing.T,
	ctx context.Context,
	db *sql.DB,
	cfg config.DatabaseConfig,
	seedOffset byte,
) {
	t.Helper()

	baseCount := m14LocaleCount(t, ctx, db)
	invalidNames := []string{"", " Leading", "Trailing ", strings.Repeat("n", 129)}
	for index, name := range invalidNames {
		localeID := m14LocaleID(t, cfg, byte(40+index)+seedOffset)
		_, err := db.ExecContext(
			ctx,
			m14LocaleInsertSQL(cfg),
			localeID,
			fmt.Sprintf("zz-name-%d", index),
			name,
			"ltr",
		)
		require.Error(t, err, "invalid native display name %d must be rejected", index)
		m14RequireLocaleIDCount(t, ctx, db, cfg, localeID, 0)
	}

	maximumNameID := m14LocaleID(t, cfg, 44+seedOffset)
	m14InsertLocale(
		t,
		ctx,
		db,
		cfg,
		maximumNameID,
		"zz-name-max",
		strings.Repeat("ก", 128),
		"ltr",
	)
	m14RequireLocaleIDCount(t, ctx, db, cfg, maximumNameID, 1)
	_, err := db.ExecContext(
		ctx,
		"DELETE FROM locales WHERE id = "+migrationPlaceholder(cfg, 1),
		maximumNameID,
	)
	require.NoError(t, err)
	require.Equal(t, baseCount, m14LocaleCount(t, ctx, db))
}

func m14AssertDirectionConstraints(
	t *testing.T,
	ctx context.Context,
	db *sql.DB,
	cfg config.DatabaseConfig,
	seedOffset byte,
) {
	t.Helper()

	baseCount := m14LocaleCount(t, ctx, db)
	for index, direction := range []string{"", "LTR", "ltr ", "auto", "rtl\x00"} {
		localeID := m14LocaleID(t, cfg, byte(50+index)+seedOffset)
		_, err := db.ExecContext(
			ctx,
			m14LocaleInsertSQL(cfg),
			localeID,
			fmt.Sprintf("zz-direction-%d", index),
			fmt.Sprintf("Invalid direction %d", index),
			direction,
		)
		require.Error(t, err, "invalid direction %d must be rejected", index)
		m14RequireLocaleIDCount(t, ctx, db, cfg, localeID, 0)
	}
	require.Equal(t, baseCount, m14LocaleCount(t, ctx, db))
}

func m14AssertMySQLNonStrictLocaleBoundaries(
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

	tests := []struct {
		id        any
		statement string
		arguments []any
	}{
		{
			id:        m14LocaleID(t, cfg, 60+seedOffset),
			statement: "INSERT INTO locales (id, tag, name, direction) VALUES (" + migrationPlaceholders(cfg, 4) + ")",
			arguments: []any{m14LocaleID(t, cfg, 60+seedOffset), strings.Repeat("a-", 256) + "a", "Overlong tag", "ltr"},
		},
		{
			id:        m14LocaleID(t, cfg, 61+seedOffset),
			statement: "INSERT INTO locales (id, tag, name, direction) VALUES (" + migrationPlaceholders(cfg, 4) + ")",
			arguments: []any{m14LocaleID(t, cfg, 61+seedOffset), "zz-nonstrict-name", strings.Repeat("n", 257), "ltr"},
		},
		{
			id:        m14LocaleID(t, cfg, 62+seedOffset),
			statement: "INSERT INTO locales (id, tag, name, direction) VALUES (" + migrationPlaceholders(cfg, 4) + ")",
			arguments: []any{m14LocaleID(t, cfg, 62+seedOffset), "zz-nonstrict-direction", "Direction truncation", "ltr             x"},
		},
		{
			id:        m14LocaleID(t, cfg, 63+seedOffset),
			statement: "INSERT INTO locales (id, tag, name, direction, enabled) VALUES (" + migrationPlaceholders(cfg, 5) + ")",
			arguments: []any{m14LocaleID(t, cfg, 63+seedOffset), "zz-nonstrict-enabled", "Invalid enabled", "ltr", 2},
		},
		{
			id: m14LocaleID(t, cfg, 64+seedOffset),
			statement: "INSERT INTO locales (id, tag, name, direction, enabled, default_slot) VALUES (" +
				migrationPlaceholders(cfg, 6) + ")",
			arguments: []any{m14LocaleID(t, cfg, 64+seedOffset), "zz-nonstrict-default", "Invalid default", "ltr", 1, 2},
		},
	}
	for index, test := range tests {
		_, err := connection.ExecContext(ctx, test.statement, test.arguments...)
		require.Error(t, err, "non-strict boundary %d must be rejected", index)
		var count int
		require.NoError(t, connection.QueryRowContext(
			ctx,
			"SELECT COUNT(*) FROM locales WHERE id = "+migrationPlaceholder(cfg, 1),
			test.id,
		).Scan(&count))
		require.Zero(t, count)
	}
}
