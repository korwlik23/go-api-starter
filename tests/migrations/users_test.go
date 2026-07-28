package migrations_test

import (
	"bytes"
	"context"
	"database/sql"
	"fmt"
	"testing"
	"time"

	"github.com/korwlik23/go-api-starter/internal/app"
	"github.com/korwlik23/go-api-starter/internal/platform/config"
	"github.com/korwlik23/go-api-starter/internal/platform/database"
	"github.com/stretchr/testify/require"
)

const (
	m3UsersTable        = "users"
	m3MigrationFilename = "000001_create_users.sql"
	m3VersionTable      = "goose_identity_version"
)

func TestM3(t *testing.T) {
	ctx, cfg, sqlDB := openMigrationTestDatabase(t)
	m3CleanupSchema(t, sqlDB)
	t.Cleanup(func() { m3CleanupSchema(t, sqlDB) })

	provider := newMigrationTestProvider(
		t,
		sqlDB,
		cfg,
		migrationTestFileBundle(
			t,
			"identity",
			migrationTestDialect(t, cfg),
			m3MigrationFilename,
			false,
		),
		m3VersionTable,
	)

	_, err := provider.UpTo(ctx, 1)
	require.NoError(t, err)
	m3AssertVersion(t, ctx, provider, 1)
	m3AssertUsersSchema(t, ctx, sqlDB, cfg)

	_, err = provider.DownTo(ctx, 0)
	require.NoError(t, err)
	require.False(t, migrationTableExists(t, ctx, sqlDB, cfg, m3UsersTable))
	m3AssertVersion(t, ctx, provider, 0)

	_, err = provider.UpTo(ctx, 1)
	require.NoError(t, err)
	m3AssertUsersSchema(t, ctx, sqlDB, cfg)
}

func TestM3Parity(t *testing.T) {
	t.Parallel()

	module := database.MigrationModule{
		ID:      "identity",
		Bundles: strictMigrationTestBundles(t, "identity"),
	}
	require.NoError(t, app.ValidateMigrationDialectParity(
		[]database.MigrationModule{module},
	))
}

func m3AssertVersion(
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

func m3AssertUsersSchema(
	t *testing.T,
	ctx context.Context,
	db *sql.DB,
	cfg config.DatabaseConfig,
) {
	t.Helper()

	columns := migrationColumns(t, ctx, db, cfg, m3UsersTable)
	require.NotEmpty(t, columns, "users table must exist")
	require.Len(t, columns, 6, "M3 must create only the approved user columns")
	m3AssertRequiredColumn(t, columns, "id", m3ExpectedIDType(t, cfg), false)
	m3AssertRequiredColumn(
		t,
		columns,
		"normalized_email",
		m3ExpectedVarcharType(t, cfg),
		false,
	)
	m3AssertRequiredColumn(t, columns, "status", m3ExpectedVarcharType(t, cfg), false)
	m3AssertRequiredColumn(t, columns, "authorization_version", "bigint", true)
	m3AssertRequiredColumn(
		t,
		columns,
		"created_at",
		m3ExpectedTimestampType(t, cfg),
		true,
	)
	m3AssertRequiredColumn(
		t,
		columns,
		"updated_at",
		m3ExpectedTimestampType(t, cfg),
		true,
	)
	require.Equal(t, int64(320), columns["normalized_email"].maximumLength.Int64)
	if cfg.Driver == "mysql" {
		require.Equal(t, int64(16), columns["id"].maximumLength.Int64)
	}
	require.True(t, migrationHasExactConstraintColumns(
		t,
		ctx,
		db,
		cfg,
		m3UsersTable,
		"PRIMARY KEY",
		[]string{"id"},
	))
	require.True(t, migrationHasExactConstraintColumns(
		t,
		ctx,
		db,
		cfg,
		m3UsersTable,
		"UNIQUE",
		[]string{"normalized_email"},
	))

	firstID := m3UserID(t, cfg, 1)
	secondID := m3UserID(t, cfg, 2)
	insertSQL := "INSERT INTO users (id, normalized_email, status) VALUES (" +
		migrationPlaceholders(cfg, 3) + ")"
	_, err := db.ExecContext(
		ctx,
		insertSQL,
		firstID,
		"first@example.test",
		"test-status",
	)
	require.NoError(t, err)
	_, err = db.ExecContext(
		ctx,
		insertSQL,
		secondID,
		"first@example.test",
		"different-status",
	)
	require.Error(t, err, "duplicate normalized email must be rejected")
	_, err = db.ExecContext(
		ctx,
		insertSQL,
		firstID,
		"second@example.test",
		"test-status",
	)
	require.Error(t, err, "duplicate user ID must be rejected")
	_, err = db.ExecContext(
		ctx,
		"INSERT INTO users (normalized_email, status) VALUES ("+
			migrationPlaceholders(cfg, 2)+")",
		"missing-id@example.test",
		"test-status",
	)
	require.Error(t, err, "user ID must not have a database default")
	_, err = db.ExecContext(
		ctx,
		"INSERT INTO users (id, status) VALUES ("+
			migrationPlaceholders(cfg, 2)+")",
		secondID,
		"test-status",
	)
	require.Error(t, err, "normalized email must be explicit")
	_, err = db.ExecContext(
		ctx,
		"INSERT INTO users (id, normalized_email) VALUES ("+
			migrationPlaceholders(cfg, 2)+")",
		secondID,
		"missing-status@example.test",
	)
	require.Error(t, err, "user status must be explicit")
	_, err = db.ExecContext(
		ctx,
		"INSERT INTO users (id, normalized_email, status, authorization_version) "+
			"VALUES ("+migrationPlaceholders(cfg, 4)+")",
		secondID,
		"invalid-version@example.test",
		"test-status",
		0,
	)
	require.Error(t, err, "authorization version must remain positive")

	var authorizationVersion int64
	var createdAt time.Time
	var updatedAt time.Time
	err = db.QueryRowContext(
		ctx,
		"SELECT authorization_version, created_at, updated_at "+
			"FROM users WHERE normalized_email = "+migrationPlaceholder(cfg, 1),
		"first@example.test",
	).Scan(&authorizationVersion, &createdAt, &updatedAt)
	require.NoError(t, err)
	require.Equal(t, int64(1), authorizationVersion)
	require.False(t, createdAt.IsZero())
	require.False(t, updatedAt.IsZero())
}

func m3AssertRequiredColumn(
	t *testing.T,
	columns map[string]migrationColumn,
	name string,
	expectedType string,
	requiresDefault bool,
) {
	t.Helper()

	column, exists := columns[name]
	require.True(t, exists, "required column %s is missing", name)
	require.Equal(t, expectedType, column.dataType)
	require.Equal(t, "NO", column.nullable)
	require.Equal(t, requiresDefault, column.defaultValue.Valid, "unexpected default for %s", name)
}

func m3CleanupSchema(t *testing.T, db *sql.DB) {
	t.Helper()

	cleanupContext, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	for _, table := range []string{m3UsersTable, m3VersionTable} {
		_, err := db.ExecContext(cleanupContext, "DROP TABLE IF EXISTS "+table)
		require.NoError(t, err)
	}
}

func m3ExpectedIDType(t *testing.T, cfg config.DatabaseConfig) string {
	t.Helper()

	if cfg.Driver == "postgres" {
		return "uuid"
	}
	if cfg.Driver == "mysql" {
		return "binary"
	}
	t.Fatalf("unsupported M3 database profile")
	return ""
}

func m3ExpectedVarcharType(t *testing.T, cfg config.DatabaseConfig) string {
	t.Helper()

	if cfg.Driver == "postgres" {
		return "character varying"
	}
	if cfg.Driver == "mysql" {
		return "varchar"
	}
	t.Fatalf("unsupported M3 database profile")
	return ""
}

func m3ExpectedTimestampType(t *testing.T, cfg config.DatabaseConfig) string {
	t.Helper()

	if cfg.Driver == "postgres" {
		return "timestamp with time zone"
	}
	if cfg.Driver == "mysql" {
		return "datetime"
	}
	t.Fatalf("unsupported M3 database profile")
	return ""
}

func m3UserID(t *testing.T, cfg config.DatabaseConfig, seed byte) any {
	t.Helper()

	if cfg.Driver == "postgres" {
		return fmt.Sprintf("%08x-0000-4000-8000-%012x", seed, seed)
	}
	if cfg.Driver == "mysql" {
		return bytes.Repeat([]byte{seed}, 16)
	}
	t.Fatalf("unsupported M3 database profile")
	return nil
}
