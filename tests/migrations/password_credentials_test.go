package migrations_test

import (
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
	m4CredentialsTable  = "password_credentials"
	m4MigrationFilename = "000002_create_password_credentials.sql"
	m4PHCHashOne        = "$argon2id$v=19$m=65536,t=3,p=1$" +
		"bm90LXNlY3JldC1zYWx0$bm90LXNlY3JldC1oYXNoLW9uZQ"
	m4PHCHashTwo = "$argon2id$v=19$m=65536,t=3,p=1$" +
		"bm90LXNlY3JldC1zYWx0$bm90LXNlY3JldC1oYXNoLXR3bw"
	m4PHCHashThree = "$argon2id$v=19$m=65536,t=3,p=1$" +
		"bm90LXNlY3JldC1zYWx0$bm90LXNlY3JldC1oYXNoLXRocmVl"
	m4PHCHashFour = "$argon2id$v=19$m=65536,t=3,p=1$" +
		"bm90LXNlY3JldC1zYWx0$bm90LXNlY3JldC1oYXNoLWZvdXI"
	m4PHCHashUpdated = "$argon2id$v=19$m=65536,t=3,p=1$" +
		"bm90LXNlY3JldC1zYWx0$bm90LXNlY3JldC1oYXNoLXVwZGF0ZWQ"
)

func TestM4(t *testing.T) {
	ctx, cfg, sqlDB := openMigrationTestDatabase(t)
	m4CleanupSchema(t, sqlDB)
	t.Cleanup(func() { m4CleanupSchema(t, sqlDB) })

	bundle := migrationTestFilesBundle(
		t,
		"identity",
		migrationTestDialect(t, cfg),
		m3MigrationFilename,
		m4MigrationFilename,
	)
	provider := newMigrationTestProvider(t, sqlDB, cfg, bundle, m3VersionTable)

	_, err := provider.UpTo(ctx, 2)
	require.NoError(t, err)
	m4AssertCredentialsSchema(t, ctx, sqlDB, cfg)
	m4AssertVersion(t, ctx, provider, 2)
	m4AssertCredentialBehavior(t, ctx, sqlDB, cfg, 0)

	_, err = provider.DownTo(ctx, 1)
	require.NoError(t, err)
	require.False(t, migrationTableExists(t, ctx, sqlDB, cfg, m4CredentialsTable))
	require.True(t, migrationTableExists(t, ctx, sqlDB, cfg, m3UsersTable))
	m4AssertVersion(t, ctx, provider, 1)

	_, err = provider.UpTo(ctx, 2)
	require.NoError(t, err)
	m4AssertVersion(t, ctx, provider, 2)
	m4AssertCredentialsSchema(t, ctx, sqlDB, cfg)
	m4AssertCredentialBehavior(t, ctx, sqlDB, cfg, 50)
}

func TestM4Parity(t *testing.T) {
	t.Parallel()

	module := database.MigrationModule{
		ID:      "identity",
		Bundles: strictMigrationTestBundles(t, "identity"),
	}
	require.NoError(t, app.ValidateMigrationDialectParity(
		[]database.MigrationModule{module},
	))
}

func m4AssertVersion(
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

func m4AssertCredentialsSchema(
	t *testing.T,
	ctx context.Context,
	db *sql.DB,
	cfg config.DatabaseConfig,
) {
	t.Helper()

	columns := migrationColumns(t, ctx, db, cfg, m4CredentialsTable)
	require.NotEmpty(t, columns, "password credentials table must exist")
	require.Len(t, columns, 3, "M4 must persist only user, hash, and change time")
	m4AssertColumn(t, columns, "user_id", m3ExpectedIDType(t, cfg), false)
	m4AssertColumn(t, columns, "password_hash", m3ExpectedVarcharType(t, cfg), false)
	m4AssertColumn(t, columns, "changed_at", m3ExpectedTimestampType(t, cfg), true)
	require.Equal(t, int64(255), columns["password_hash"].maximumLength.Int64)
	if cfg.Driver == "mysql" {
		require.Equal(t, int64(16), columns["user_id"].maximumLength.Int64)
	}
	require.True(t, migrationHasExactConstraintColumns(
		t,
		ctx,
		db,
		cfg,
		m4CredentialsTable,
		"PRIMARY KEY",
		[]string{"user_id"},
	))
	require.True(t, migrationHasExactForeignKey(
		t,
		ctx,
		db,
		cfg,
		m4CredentialsTable,
		migrationForeignKey{
			localColumns:      []string{"user_id"},
			referencedTable:   m3UsersTable,
			referencedColumns: []string{"id"},
			updateRule:        "RESTRICT",
			deleteRule:        "CASCADE",
		},
	))
}

func m4AssertCredentialBehavior(
	t *testing.T,
	ctx context.Context,
	db *sql.DB,
	cfg config.DatabaseConfig,
	seedOffset byte,
) {
	t.Helper()

	userID := m3UserID(t, cfg, 40+seedOffset)
	m2InsertUser(
		t,
		ctx,
		db,
		cfg,
		userID,
		fmt.Sprintf("credential-%d@example.test", seedOffset),
	)
	userWithoutHash := m3UserID(t, cfg, 41+seedOffset)
	m2InsertUser(
		t,
		ctx,
		db,
		cfg,
		userWithoutHash,
		fmt.Sprintf("credential-without-hash-%d@example.test", seedOffset),
	)
	insertSQL := "INSERT INTO password_credentials (user_id, password_hash) VALUES (" +
		migrationPlaceholders(cfg, 2) + ")"
	firstHash := m4PHCHashOne
	_, err := db.ExecContext(ctx, insertSQL, userID, firstHash)
	require.NoError(t, err)
	_, err = db.ExecContext(
		ctx,
		insertSQL,
		userID,
		m4PHCHashTwo,
	)
	require.Error(t, err, "one user must have at most one password credential")
	_, err = db.ExecContext(
		ctx,
		insertSQL,
		m3UserID(t, cfg, 99+seedOffset),
		m4PHCHashThree,
	)
	require.Error(t, err, "credential for an unknown user must be rejected")
	_, err = db.ExecContext(
		ctx,
		"INSERT INTO password_credentials (password_hash) VALUES ("+
			migrationPlaceholder(cfg, 1)+")",
		m4PHCHashFour,
	)
	require.Error(t, err, "credential user must be explicit")
	_, err = db.ExecContext(
		ctx,
		"INSERT INTO password_credentials (user_id) VALUES ("+
			migrationPlaceholder(cfg, 1)+")",
		userWithoutHash,
	)
	require.Error(t, err, "password hash must be explicit")

	var storedHash string
	var changedAt time.Time
	err = db.QueryRowContext(
		ctx,
		"SELECT password_hash, changed_at FROM password_credentials "+
			"WHERE user_id = "+migrationPlaceholder(cfg, 1),
		userID,
	).Scan(&storedHash, &changedAt)
	require.NoError(t, err)
	require.Equal(t, firstHash, storedHash)
	require.NotEqual(t, "raw-password-fixture", storedHash)
	require.False(t, changedAt.IsZero())

	initialChangedAt := changedAt
	nextChangedAt := initialChangedAt.Add(time.Second)
	secondHash := m4PHCHashUpdated
	_, err = db.ExecContext(
		ctx,
		"UPDATE password_credentials SET password_hash = "+
			migrationPlaceholder(cfg, 1)+", changed_at = "+
			migrationPlaceholder(cfg, 2)+" WHERE user_id = "+
			migrationPlaceholder(cfg, 3),
		secondHash,
		nextChangedAt,
		userID,
	)
	require.NoError(t, err)
	err = db.QueryRowContext(
		ctx,
		"SELECT password_hash, changed_at FROM password_credentials "+
			"WHERE user_id = "+migrationPlaceholder(cfg, 1),
		userID,
	).Scan(&storedHash, &changedAt)
	require.NoError(t, err)
	require.Equal(t, secondHash, storedHash)
	require.WithinDuration(t, nextChangedAt, changedAt, time.Microsecond)
	require.True(t, changedAt.After(initialChangedAt))

	_, err = db.ExecContext(
		ctx,
		"DELETE FROM users WHERE id = "+migrationPlaceholder(cfg, 1),
		userID,
	)
	require.NoError(t, err)
	var credentialCount int
	require.NoError(t, db.QueryRowContext(
		ctx,
		"SELECT COUNT(*) FROM password_credentials WHERE user_id = "+
			migrationPlaceholder(cfg, 1),
		userID,
	).Scan(&credentialCount))
	require.Zero(t, credentialCount, "credential must be removed with its user")
}

func m4AssertColumn(
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

func m4CleanupSchema(t *testing.T, db *sql.DB) {
	t.Helper()

	cleanupContext, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	for _, table := range []string{
		m4CredentialsTable,
		m3UsersTable,
		m3VersionTable,
	} {
		_, err := db.ExecContext(cleanupContext, "DROP TABLE IF EXISTS "+table)
		require.NoError(t, err)
	}
}
