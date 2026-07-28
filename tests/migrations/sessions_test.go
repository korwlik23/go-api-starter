package migrations_test

import (
	"context"
	"database/sql"
	"strings"
	"testing"
	"time"

	"github.com/korwlik23/go-api-starter/internal/app"
	"github.com/korwlik23/go-api-starter/internal/platform/config"
	"github.com/korwlik23/go-api-starter/internal/platform/database"
	"github.com/stretchr/testify/require"
)

const (
	m5SessionsTable     = "sessions"
	m5MigrationFilename = "000003_create_sessions.sql"
)

func TestM5(t *testing.T) {
	ctx, cfg, sqlDB := openMigrationTestDatabase(t)
	m5CleanupSchema(t, sqlDB)
	t.Cleanup(func() { m5CleanupSchema(t, sqlDB) })

	bundle := migrationTestFilesBundle(
		t,
		"identity",
		migrationTestDialect(t, cfg),
		m3MigrationFilename,
		m4MigrationFilename,
		m5MigrationFilename,
	)
	provider := newMigrationTestProvider(t, sqlDB, cfg, bundle, m3VersionTable)

	_, err := provider.UpTo(ctx, 3)
	require.NoError(t, err)
	m5AssertSessionsSchema(t, ctx, sqlDB, cfg)
	m5AssertVersion(t, ctx, provider, 3)
	m5AssertSessionBehavior(t, ctx, sqlDB, cfg, 0)

	_, err = provider.DownTo(ctx, 2)
	require.NoError(t, err)
	require.False(t, migrationTableExists(t, ctx, sqlDB, cfg, m5SessionsTable))
	require.True(t, migrationTableExists(t, ctx, sqlDB, cfg, m4CredentialsTable))
	require.True(t, migrationTableExists(t, ctx, sqlDB, cfg, m3UsersTable))
	m5AssertVersion(t, ctx, provider, 2)

	_, err = provider.UpTo(ctx, 3)
	require.NoError(t, err)
	m5AssertVersion(t, ctx, provider, 3)
	m5AssertSessionsSchema(t, ctx, sqlDB, cfg)
	m5AssertSessionBehavior(t, ctx, sqlDB, cfg, 60)
}

func TestM5Parity(t *testing.T) {
	t.Parallel()

	module := database.MigrationModule{
		ID:      "identity",
		Bundles: strictMigrationTestBundles(t, "identity"),
	}
	require.NoError(t, app.ValidateMigrationDialectParity(
		[]database.MigrationModule{module},
	))
}

func m5AssertVersion(
	t *testing.T,
	ctx context.Context,
	provider interface {
		GetDBVersion(context.Context) (int64, error)
	},
	expected int64,
) {
	t.Helper()

	version, err := provider.GetDBVersion(ctx)
	require.NoError(t, err)
	require.Equal(t, expected, version)
}

func m5AssertSessionsSchema(
	t *testing.T,
	ctx context.Context,
	db *sql.DB,
	cfg config.DatabaseConfig,
) {
	t.Helper()

	columns := migrationColumns(t, ctx, db, cfg, m5SessionsTable)
	require.NotEmpty(t, columns, "sessions table must exist")
	require.Len(t, columns, 10, "M5 must create only approved session columns")
	m5AssertColumn(t, columns, "id", m3ExpectedIDType(t, cfg), "NO", false)
	m5AssertColumn(t, columns, "token_hash", m5ExpectedHashType(t, cfg), "NO", false)
	m5AssertColumn(t, columns, "csrf_token_hash", m5ExpectedHashType(t, cfg), "NO", false)
	m5AssertColumn(t, columns, "state", m3ExpectedVarcharType(t, cfg), "NO", false)
	m5AssertColumn(t, columns, "user_id", m3ExpectedIDType(t, cfg), "YES", false)
	m5AssertColumn(t, columns, "authorization_version", "bigint", "YES", false)
	m5AssertColumn(
		t,
		columns,
		"idle_expires_at",
		m3ExpectedTimestampType(t, cfg),
		"NO",
		false,
	)
	m5AssertColumn(
		t,
		columns,
		"absolute_expires_at",
		m3ExpectedTimestampType(t, cfg),
		"NO",
		false,
	)
	m5AssertColumn(
		t,
		columns,
		"revoked_at",
		m3ExpectedTimestampType(t, cfg),
		"YES",
		false,
	)
	m5AssertColumn(
		t,
		columns,
		"created_at",
		m3ExpectedTimestampType(t, cfg),
		"NO",
		true,
	)
	require.Equal(t, int64(32), columns["state"].maximumLength.Int64)
	if cfg.Driver == "mysql" {
		for _, column := range []string{"id", "user_id"} {
			require.Equal(t, int64(16), columns[column].maximumLength.Int64)
		}
		for _, column := range []string{"token_hash", "csrf_token_hash"} {
			require.Equal(t, int64(64), columns[column].maximumLength.Int64)
		}
	}

	require.True(t, migrationHasExactConstraintColumns(
		t, ctx, db, cfg, m5SessionsTable, "PRIMARY KEY", []string{"id"},
	))
	require.True(t, migrationHasExactConstraintColumns(
		t,
		ctx,
		db,
		cfg,
		m5SessionsTable,
		"UNIQUE",
		[]string{"token_hash"},
	))
	require.True(t, migrationHasExactForeignKey(
		t,
		ctx,
		db,
		cfg,
		m5SessionsTable,
		migrationForeignKey{
			localColumns:      []string{"user_id"},
			referencedTable:   m3UsersTable,
			referencedColumns: []string{"id"},
			updateRule:        "RESTRICT",
			deleteRule:        "CASCADE",
		},
	))
	require.True(t, migrationHasExactIndexColumns(
		t,
		ctx,
		db,
		cfg,
		m5SessionsTable,
		false,
		[]string{"user_id"},
	))
	require.True(t, migrationHasExactIndexColumns(
		t,
		ctx,
		db,
		cfg,
		m5SessionsTable,
		false,
		[]string{"absolute_expires_at"},
	))
}

func m5AssertColumn(
	t *testing.T,
	columns map[string]migrationColumn,
	name string,
	expectedType string,
	expectedNullable string,
	requiresDefault bool,
) {
	t.Helper()

	column, exists := columns[name]
	require.True(t, exists, "required column %s is missing", name)
	require.Equal(t, expectedType, column.dataType)
	require.Equal(t, expectedNullable, column.nullable)
	hasMaterialDefault := column.defaultValue.Valid &&
		!strings.EqualFold(strings.TrimSpace(column.defaultValue.String), "NULL")
	require.Equal(t, requiresDefault, hasMaterialDefault, "unexpected default for %s", name)
}

func m5ExpectedHashType(t *testing.T, cfg config.DatabaseConfig) string {
	t.Helper()

	if cfg.Driver == "postgres" {
		return "bytea"
	}
	if cfg.Driver == "mysql" {
		return "varbinary"
	}
	t.Fatalf("unsupported M5 database profile")
	return ""
}

func m5CleanupSchema(t *testing.T, db *sql.DB) {
	t.Helper()

	cleanupContext, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	for _, table := range []string{
		m5SessionsTable,
		m4CredentialsTable,
		m3UsersTable,
		m3VersionTable,
	} {
		_, err := db.ExecContext(cleanupContext, "DROP TABLE IF EXISTS "+table)
		require.NoError(t, err)
	}
}
