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
	m6AuthAttemptsTable = "auth_attempts"
	m6MigrationFilename = "000004_create_auth_attempts.sql"
)

func TestM6(t *testing.T) {
	ctx, cfg, sqlDB := openMigrationTestDatabase(t)
	m6CleanupSchema(t, sqlDB)
	t.Cleanup(func() { m6CleanupSchema(t, sqlDB) })

	bundle := migrationTestFilesBundle(
		t,
		"identity",
		migrationTestDialect(t, cfg),
		m3MigrationFilename,
		m4MigrationFilename,
		m5MigrationFilename,
		m6MigrationFilename,
	)
	provider := newMigrationTestProvider(t, sqlDB, cfg, bundle, m3VersionTable)

	_, err := provider.UpTo(ctx, 4)
	require.NoError(t, err)
	m6AssertAuthAttemptsSchema(t, ctx, sqlDB, cfg)
	m6AssertVersion(t, ctx, provider, 4)
	m6AssertAuthAttemptBehavior(t, ctx, sqlDB, cfg, 0)

	_, err = provider.DownTo(ctx, 3)
	require.NoError(t, err)
	require.False(t, migrationTableExists(t, ctx, sqlDB, cfg, m6AuthAttemptsTable))
	require.True(t, migrationTableExists(t, ctx, sqlDB, cfg, m5SessionsTable))
	require.True(t, migrationTableExists(t, ctx, sqlDB, cfg, m4CredentialsTable))
	require.True(t, migrationTableExists(t, ctx, sqlDB, cfg, m3UsersTable))
	m6AssertVersion(t, ctx, provider, 3)

	_, err = provider.UpTo(ctx, 4)
	require.NoError(t, err)
	m6AssertVersion(t, ctx, provider, 4)
	m6AssertAuthAttemptsSchema(t, ctx, sqlDB, cfg)
	m6AssertAuthAttemptBehavior(t, ctx, sqlDB, cfg, 100)
}

func TestM6Parity(t *testing.T) {
	t.Parallel()

	module := database.MigrationModule{
		ID:      "identity",
		Bundles: strictMigrationTestBundles(t, "identity"),
	}
	require.NoError(t, app.ValidateMigrationDialectParity(
		[]database.MigrationModule{module},
	))
}

func m6AssertVersion(
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

func m6AssertAuthAttemptsSchema(
	t *testing.T,
	ctx context.Context,
	db *sql.DB,
	cfg config.DatabaseConfig,
) {
	t.Helper()

	columns := migrationColumns(t, ctx, db, cfg, m6AuthAttemptsTable)
	require.NotEmpty(t, columns, "auth attempts table must exist")
	require.Len(t, columns, 9, "M6 must persist only keyed operational lockout state")
	m6AssertColumn(t, columns, "key_type", m3ExpectedVarcharType(t, cfg), "NO", false)
	m6AssertColumn(t, columns, "key_hash", m6ExpectedHashType(t, cfg), "NO", false)
	m6AssertColumn(t, columns, "failure_count", "bigint", "NO", false)
	m6AssertColumn(
		t,
		columns,
		"window_started_at",
		m3ExpectedTimestampType(t, cfg),
		"NO",
		false,
	)
	m6AssertColumn(
		t,
		columns,
		"window_expires_at",
		m3ExpectedTimestampType(t, cfg),
		"NO",
		false,
	)
	m6AssertColumn(
		t,
		columns,
		"last_failed_at",
		m3ExpectedTimestampType(t, cfg),
		"NO",
		false,
	)
	m6AssertColumn(
		t,
		columns,
		"locked_until",
		m3ExpectedTimestampType(t, cfg),
		"YES",
		false,
	)
	m6AssertColumn(
		t,
		columns,
		"created_at",
		m3ExpectedTimestampType(t, cfg),
		"NO",
		true,
	)
	m6AssertColumn(
		t,
		columns,
		"updated_at",
		m3ExpectedTimestampType(t, cfg),
		"NO",
		true,
	)
	require.Equal(t, int64(16), columns["key_type"].maximumLength.Int64)
	if cfg.Driver == "mysql" {
		require.Equal(t, int64(64), columns["key_hash"].maximumLength.Int64)
	}

	require.True(t, migrationHasExactConstraintColumns(
		t,
		ctx,
		db,
		cfg,
		m6AuthAttemptsTable,
		"PRIMARY KEY",
		[]string{"key_type", "key_hash"},
	))
	require.True(t, migrationHasExactIndexColumns(
		t,
		ctx,
		db,
		cfg,
		m6AuthAttemptsTable,
		false,
		[]string{"window_expires_at"},
	))
	require.Empty(
		t,
		migrationForeignKeys(t, ctx, db, cfg, m6AuthAttemptsTable),
		"auth attempts must include unknown subjects without a users foreign key",
	)
}

func m6AssertColumn(
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

func m6ExpectedHashType(t *testing.T, cfg config.DatabaseConfig) string {
	t.Helper()

	if cfg.Driver == "postgres" {
		return "bytea"
	}
	if cfg.Driver == "mysql" {
		return "varbinary"
	}
	t.Fatalf("unsupported M6 database profile")
	return ""
}

func m6CleanupSchema(t *testing.T, db *sql.DB) {
	t.Helper()

	cleanupContext, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	for _, table := range []string{
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
