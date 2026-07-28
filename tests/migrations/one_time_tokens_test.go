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
	m8TokensTable       = "one_time_tokens"
	m8MigrationFilename = "000006_create_one_time_tokens.sql"
)

func TestM8(t *testing.T) {
	ctx, cfg, sqlDB := openMigrationTestDatabase(t)
	m8CleanupSchema(t, sqlDB)
	t.Cleanup(func() { m8CleanupSchema(t, sqlDB) })

	bundle := migrationTestFilesBundle(
		t,
		"identity",
		migrationTestDialect(t, cfg),
		m3MigrationFilename,
		m4MigrationFilename,
		m5MigrationFilename,
		m6MigrationFilename,
		m7MigrationFilename,
		m8MigrationFilename,
	)
	provider := newMigrationTestProvider(t, sqlDB, cfg, bundle, m3VersionTable)

	_, err := provider.UpTo(ctx, 6)
	require.NoError(t, err)
	m8AssertTokenSchema(t, ctx, sqlDB, cfg)
	m8AssertVersion(t, ctx, provider, 6)
	m8AssertTokenBehavior(t, ctx, sqlDB, cfg, 0)

	_, err = provider.DownTo(ctx, 5)
	require.NoError(t, err)
	require.False(t, migrationTableExists(t, ctx, sqlDB, cfg, m8TokensTable))
	require.True(t, migrationTableExists(t, ctx, sqlDB, cfg, m7RecoveryCodesTable))
	require.True(t, migrationTableExists(t, ctx, sqlDB, cfg, m7TOTPTable))
	require.True(t, migrationTableExists(t, ctx, sqlDB, cfg, m6AuthAttemptsTable))
	require.True(t, migrationTableExists(t, ctx, sqlDB, cfg, m5SessionsTable))
	require.True(t, migrationTableExists(t, ctx, sqlDB, cfg, m4CredentialsTable))
	require.True(t, migrationTableExists(t, ctx, sqlDB, cfg, m3UsersTable))
	m8AssertVersion(t, ctx, provider, 5)

	_, err = provider.UpTo(ctx, 6)
	require.NoError(t, err)
	m8AssertTokenSchema(t, ctx, sqlDB, cfg)
	m8AssertVersion(t, ctx, provider, 6)
	m8AssertTokenBehavior(t, ctx, sqlDB, cfg, 100)
}

func TestM8Parity(t *testing.T) {
	t.Parallel()

	module := database.MigrationModule{
		ID:      "identity",
		Bundles: strictMigrationTestBundles(t, "identity"),
	}
	require.NoError(t, app.ValidateMigrationDialectParity(
		[]database.MigrationModule{module},
	))
}

func m8AssertVersion(
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

func m8AssertTokenSchema(
	t *testing.T,
	ctx context.Context,
	db *sql.DB,
	cfg config.DatabaseConfig,
) {
	t.Helper()

	columns := migrationColumns(t, ctx, db, cfg, m8TokensTable)
	require.NotEmpty(t, columns, "one-time tokens table must exist")
	require.Len(t, columns, 10, "M8 must persist only versioned token hashes and lifecycle state")
	m8AssertColumn(t, columns, "id", m3ExpectedIDType(t, cfg), "NO", false)
	m8AssertColumn(t, columns, "user_id", m3ExpectedIDType(t, cfg), "NO", false)
	m8AssertColumn(t, columns, "purpose", m3ExpectedVarcharType(t, cfg), "NO", false)
	m8AssertColumn(t, columns, "token_hash", m8ExpectedHashType(t, cfg), "NO", false)
	m8AssertColumn(t, columns, "hash_version", "smallint", "NO", false)
	m8AssertColumn(t, columns, "hash_key_version", "bigint", "NO", false)
	m8AssertColumn(t, columns, "expires_at", m3ExpectedTimestampType(t, cfg), "NO", false)
	m8AssertColumn(t, columns, "consumed_at", m3ExpectedTimestampType(t, cfg), "YES", false)
	m8AssertColumn(t, columns, "revoked_at", m3ExpectedTimestampType(t, cfg), "YES", false)
	m8AssertColumn(t, columns, "created_at", m3ExpectedTimestampType(t, cfg), "NO", true)
	require.Equal(t, int64(32), columns["purpose"].maximumLength.Int64)
	if cfg.Driver == "mysql" {
		for _, column := range []string{"id", "user_id"} {
			require.Equal(t, int64(16), columns[column].maximumLength.Int64)
		}
		require.Equal(t, int64(64), columns["token_hash"].maximumLength.Int64)
	}

	require.True(t, migrationHasExactConstraintColumns(
		t,
		ctx,
		db,
		cfg,
		m8TokensTable,
		"PRIMARY KEY",
		[]string{"id"},
	))
	require.True(t, migrationHasExactConstraintColumns(
		t,
		ctx,
		db,
		cfg,
		m8TokensTable,
		"UNIQUE",
		[]string{"token_hash"},
	))
	require.True(t, migrationHasExactForeignKey(
		t,
		ctx,
		db,
		cfg,
		m8TokensTable,
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
		m8TokensTable,
		false,
		[]string{"user_id", "purpose", "expires_at"},
	))
	require.True(t, migrationHasExactIndexColumns(
		t,
		ctx,
		db,
		cfg,
		m8TokensTable,
		false,
		[]string{"expires_at"},
	))
	require.True(t, migrationHasExactIndexColumns(
		t,
		ctx,
		db,
		cfg,
		m8TokensTable,
		false,
		[]string{"hash_key_version", "expires_at"},
	))
}

func m8AssertColumn(
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

func m8ExpectedHashType(t *testing.T, cfg config.DatabaseConfig) string {
	t.Helper()

	if cfg.Driver == "postgres" {
		return "bytea"
	}
	if cfg.Driver == "mysql" {
		return "varbinary"
	}
	t.Fatalf("unsupported M8 database profile")
	return ""
}

func m8CleanupSchema(t *testing.T, db *sql.DB) {
	t.Helper()

	cleanupContext, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	for _, table := range []string{
		m8TokensTable,
		m7RecoveryCodesTable,
		m7TOTPTable,
		m6AuthAttemptsTable,
		m5SessionsTable,
		m4CredentialsTable,
		m3UsersTable,
		m3VersionTable,
	} {
		_, err := db.ExecContext(cleanupContext, "DROP TABLE IF EXISTS "+table)
		require.NoError(t, err)
	}
}
