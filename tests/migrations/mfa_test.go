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
	m7TOTPTable          = "mfa_totp_credentials"
	m7RecoveryCodesTable = "mfa_recovery_codes"
	m7MigrationFilename  = "000005_create_mfa.sql"
)

func TestM7(t *testing.T) {
	ctx, cfg, sqlDB := openMigrationTestDatabase(t)
	m7CleanupSchema(t, sqlDB)
	t.Cleanup(func() { m7CleanupSchema(t, sqlDB) })

	bundle := migrationTestFilesBundle(
		t,
		"identity",
		migrationTestDialect(t, cfg),
		m3MigrationFilename,
		m4MigrationFilename,
		m5MigrationFilename,
		m6MigrationFilename,
		m7MigrationFilename,
	)
	provider := newMigrationTestProvider(t, sqlDB, cfg, bundle, m3VersionTable)

	_, err := provider.UpTo(ctx, 5)
	require.NoError(t, err)
	m7AssertMFASchema(t, ctx, sqlDB, cfg)
	m7AssertVersion(t, ctx, provider, 5)
	m7AssertMFABehavior(t, ctx, sqlDB, cfg, 0)

	_, err = provider.DownTo(ctx, 4)
	require.NoError(t, err)
	require.False(t, migrationTableExists(t, ctx, sqlDB, cfg, m7RecoveryCodesTable))
	require.False(t, migrationTableExists(t, ctx, sqlDB, cfg, m7TOTPTable))
	require.True(t, migrationTableExists(t, ctx, sqlDB, cfg, m6AuthAttemptsTable))
	require.True(t, migrationTableExists(t, ctx, sqlDB, cfg, m5SessionsTable))
	require.True(t, migrationTableExists(t, ctx, sqlDB, cfg, m4CredentialsTable))
	require.True(t, migrationTableExists(t, ctx, sqlDB, cfg, m3UsersTable))
	m7AssertVersion(t, ctx, provider, 4)

	_, err = provider.UpTo(ctx, 5)
	require.NoError(t, err)
	m7AssertVersion(t, ctx, provider, 5)
	m7AssertMFASchema(t, ctx, sqlDB, cfg)
	m7AssertMFABehavior(t, ctx, sqlDB, cfg, 100)
}

func TestM7Parity(t *testing.T) {
	t.Parallel()

	module := database.MigrationModule{
		ID:      "identity",
		Bundles: strictMigrationTestBundles(t, "identity"),
	}
	require.NoError(t, app.ValidateMigrationDialectParity(
		[]database.MigrationModule{module},
	))
}

func m7AssertVersion(
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

func m7AssertMFASchema(
	t *testing.T,
	ctx context.Context,
	db *sql.DB,
	cfg config.DatabaseConfig,
) {
	t.Helper()

	m7AssertTOTPSchema(t, ctx, db, cfg)
	m7AssertRecoveryCodeSchema(t, ctx, db, cfg)
}

func m7AssertTOTPSchema(
	t *testing.T,
	ctx context.Context,
	db *sql.DB,
	cfg config.DatabaseConfig,
) {
	t.Helper()

	columns := migrationColumns(t, ctx, db, cfg, m7TOTPTable)
	require.NotEmpty(t, columns, "TOTP credentials table must exist")
	require.Len(t, columns, 10, "M7 must persist only the encrypted TOTP envelope and lifecycle state")
	m7AssertColumn(t, columns, "user_id", m3ExpectedIDType(t, cfg), "NO", false)
	m7AssertColumn(t, columns, "secret_envelope_version", "smallint", "NO", false)
	m7AssertColumn(t, columns, "secret_key_version", "bigint", "NO", false)
	m7AssertColumn(t, columns, "secret_ciphertext", m7ExpectedBinaryType(t, cfg), "NO", false)
	m7AssertColumn(t, columns, "secret_nonce", m7ExpectedBinaryType(t, cfg), "NO", false)
	m7AssertColumn(t, columns, "enrollment_expires_at", m3ExpectedTimestampType(t, cfg), "YES", false)
	m7AssertColumn(t, columns, "confirmed_at", m3ExpectedTimestampType(t, cfg), "YES", false)
	m7AssertColumn(t, columns, "last_used_step", "bigint", "YES", false)
	m7AssertColumn(t, columns, "created_at", m3ExpectedTimestampType(t, cfg), "NO", true)
	m7AssertColumn(t, columns, "updated_at", m3ExpectedTimestampType(t, cfg), "NO", true)
	if cfg.Driver == "mysql" {
		require.Equal(t, int64(1024), columns["secret_ciphertext"].maximumLength.Int64)
		require.Equal(t, int64(32), columns["secret_nonce"].maximumLength.Int64)
		require.Equal(t, int64(16), columns["user_id"].maximumLength.Int64)
	}

	require.True(t, migrationHasExactConstraintColumns(
		t,
		ctx,
		db,
		cfg,
		m7TOTPTable,
		"PRIMARY KEY",
		[]string{"user_id"},
	))
	require.True(t, migrationHasExactConstraintColumns(
		t,
		ctx,
		db,
		cfg,
		m7TOTPTable,
		"UNIQUE",
		[]string{"secret_key_version", "secret_nonce"},
	))
	require.True(t, migrationHasExactIndexColumns(
		t,
		ctx,
		db,
		cfg,
		m7TOTPTable,
		false,
		[]string{"enrollment_expires_at"},
	))
	require.True(t, migrationHasExactForeignKey(
		t,
		ctx,
		db,
		cfg,
		m7TOTPTable,
		migrationForeignKey{
			localColumns:      []string{"user_id"},
			referencedTable:   m3UsersTable,
			referencedColumns: []string{"id"},
			updateRule:        "RESTRICT",
			deleteRule:        "CASCADE",
		},
	))
}

func m7AssertRecoveryCodeSchema(
	t *testing.T,
	ctx context.Context,
	db *sql.DB,
	cfg config.DatabaseConfig,
) {
	t.Helper()

	columns := migrationColumns(t, ctx, db, cfg, m7RecoveryCodesTable)
	require.NotEmpty(t, columns, "MFA recovery codes table must exist")
	require.Len(t, columns, 6, "M7 must never persist raw MFA recovery codes")
	m7AssertColumn(t, columns, "user_id", m3ExpectedIDType(t, cfg), "NO", false)
	m7AssertColumn(t, columns, "code_hash", m7ExpectedBinaryType(t, cfg), "NO", false)
	m7AssertColumn(t, columns, "hash_version", "smallint", "NO", false)
	m7AssertColumn(t, columns, "hash_key_version", "bigint", "NO", false)
	m7AssertColumn(t, columns, "consumed_at", m3ExpectedTimestampType(t, cfg), "YES", false)
	m7AssertColumn(t, columns, "created_at", m3ExpectedTimestampType(t, cfg), "NO", true)
	if cfg.Driver == "mysql" {
		require.Equal(t, int64(64), columns["code_hash"].maximumLength.Int64)
		require.Equal(t, int64(16), columns["user_id"].maximumLength.Int64)
	}

	require.True(t, migrationHasExactConstraintColumns(
		t,
		ctx,
		db,
		cfg,
		m7RecoveryCodesTable,
		"PRIMARY KEY",
		[]string{"user_id", "code_hash"},
	))
	require.True(t, migrationHasExactForeignKey(
		t,
		ctx,
		db,
		cfg,
		m7RecoveryCodesTable,
		migrationForeignKey{
			localColumns:      []string{"user_id"},
			referencedTable:   m7TOTPTable,
			referencedColumns: []string{"user_id"},
			updateRule:        "RESTRICT",
			deleteRule:        "CASCADE",
		},
	))
}

func m7AssertColumn(
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

func m7ExpectedBinaryType(t *testing.T, cfg config.DatabaseConfig) string {
	t.Helper()

	if cfg.Driver == "postgres" {
		return "bytea"
	}
	if cfg.Driver == "mysql" {
		return "varbinary"
	}
	t.Fatalf("unsupported M7 database profile")
	return ""
}

func m7CleanupSchema(t *testing.T, db *sql.DB) {
	t.Helper()

	cleanupContext, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	for _, table := range []string{
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
