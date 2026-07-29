package migrations_test

import (
	"context"
	"database/sql"
	"io/fs"
	"strings"
	"testing"
	"time"

	"github.com/korwlik23/go-api-starter/internal/app"
	"github.com/korwlik23/go-api-starter/internal/platform/config"
	"github.com/korwlik23/go-api-starter/internal/platform/database"
	"github.com/stretchr/testify/require"
)

const (
	m17AuditEventsTable  = "audit_events"
	m17MigrationFilename = "000001_create_audit_events.sql"
	m17VersionTable      = "goose_audit_version"
)

func TestM17(t *testing.T) {
	ctx, cfg, sqlDB := openMigrationTestDatabase(t)
	m17CleanupSchema(t, sqlDB)
	t.Cleanup(func() { m17CleanupSchema(t, sqlDB) })

	accountsProvider := newMigrationTestProvider(
		t,
		sqlDB,
		cfg,
		migrationTestFileBundle(
			t,
			"accounts",
			migrationTestDialect(t, cfg),
			m1MigrationFilename,
			false,
		),
		m1MigrationTableName,
	)
	identityProvider := newMigrationTestProvider(
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
	auditProvider := newMigrationTestProvider(
		t,
		sqlDB,
		cfg,
		migrationTestFileBundle(
			t,
			"audit",
			migrationTestDialect(t, cfg),
			m17MigrationFilename,
			true,
		),
		m17VersionTable,
	)

	_, err := accountsProvider.UpTo(ctx, 1)
	require.NoError(t, err)
	_, err = identityProvider.UpTo(ctx, 1)
	require.NoError(t, err)
	m17CreatePreexistingParents(t, ctx, sqlDB, cfg)

	_, err = auditProvider.UpTo(ctx, 1)
	require.NoError(t, err)
	m17AssertAuditEventsSchema(t, ctx, sqlDB, cfg)
	m10AssertVersion(t, ctx, auditProvider, 1)
	m10AssertVersion(t, ctx, accountsProvider, 1)
	m10AssertVersion(t, ctx, identityProvider, 1)
	m17AssertNoSeededEvents(t, ctx, sqlDB)
	m17AssertAuditEventBehavior(t, ctx, sqlDB, cfg, 0)
	m17AssertAuditEventConstraints(t, ctx, sqlDB, cfg, 0)
	m17AssertAuditEventDeletePolicy(t, ctx, sqlDB, cfg, 0)

	accountsSnapshot := m17SnapshotAccounts(t, ctx, sqlDB)
	usersSnapshot := m16SnapshotUsers(t, ctx, sqlDB)

	_, err = auditProvider.DownTo(ctx, 0)
	require.NoError(t, err)
	require.False(t, migrationTableExists(t, ctx, sqlDB, cfg, m17AuditEventsTable))
	m17AssertAccountsSchemaOnly(t, ctx, sqlDB, cfg)
	m16AssertUsersSchemaOnly(t, ctx, sqlDB, cfg)
	require.Equal(t, accountsSnapshot, m17SnapshotAccounts(t, ctx, sqlDB))
	require.Equal(t, usersSnapshot, m16SnapshotUsers(t, ctx, sqlDB))
	m10AssertVersion(t, ctx, auditProvider, 0)
	m10AssertVersion(t, ctx, accountsProvider, 1)
	m10AssertVersion(t, ctx, identityProvider, 1)

	_, err = auditProvider.UpTo(ctx, 1)
	require.NoError(t, err)
	m17AssertAuditEventsSchema(t, ctx, sqlDB, cfg)
	m17AssertNoSeededEvents(t, ctx, sqlDB)
	m17AssertAuditEventBehavior(t, ctx, sqlDB, cfg, 100)
	m17AssertAuditEventConstraints(t, ctx, sqlDB, cfg, 100)
	m17AssertAuditEventDeletePolicy(t, ctx, sqlDB, cfg, 100)
	m10AssertVersion(t, ctx, auditProvider, 1)
}

func TestM17Parity(t *testing.T) {
	t.Parallel()

	module := database.MigrationModule{
		ID:      "audit",
		Bundles: strictMigrationTestBundles(t, "audit"),
	}
	require.NoError(t, app.ValidateMigrationDialectParity(
		[]database.MigrationModule{module},
	))
}

func TestM17OracleMySQLCheckBoundForeignKeysUseImplicitRestrictiveActions(t *testing.T) {
	t.Parallel()

	bundle := migrationTestFileBundle(
		t,
		"audit",
		"mysql",
		m17MigrationFilename,
		true,
	)
	migration, err := fs.ReadFile(bundle, m17MigrationFilename)
	require.NoError(t, err)
	source := strings.ToUpper(string(migration))
	for _, constraint := range []string{
		"AUDIT_EVENTS_ACCOUNT_FK",
		"AUDIT_EVENTS_ACTOR_FK",
	} {
		clause := m17NamedConstraintClause(t, source, constraint)
		require.NotContains(t, clause, "ON UPDATE")
		require.NotContains(t, clause, "ON DELETE")
	}
}

func m17AssertAuditEventsSchema(
	t *testing.T,
	ctx context.Context,
	db *sql.DB,
	cfg config.DatabaseConfig,
) {
	t.Helper()

	columns := migrationColumns(t, ctx, db, cfg, m17AuditEventsTable)
	require.NotEmpty(t, columns, "audit_events table must exist")
	require.Len(t, columns, 11, "M17 must persist only the locked append-only envelope")
	m9AssertColumn(t, columns, "id", m3ExpectedIDType(t, cfg), "NO", false)
	m9AssertColumn(t, columns, "scope", m3ExpectedVarcharType(t, cfg), "NO", false)
	m9AssertColumn(t, columns, "account_id", m3ExpectedIDType(t, cfg), "YES", false)
	m9AssertColumn(t, columns, "actor_type", m3ExpectedVarcharType(t, cfg), "NO", false)
	m9AssertColumn(t, columns, "actor_id", m3ExpectedIDType(t, cfg), "YES", false)
	m9AssertColumn(t, columns, "action", m3ExpectedVarcharType(t, cfg), "NO", false)
	m9AssertColumn(t, columns, "operation_id", m3ExpectedIDType(t, cfg), "NO", false)
	m9AssertColumn(t, columns, "request_id", m3ExpectedVarcharType(t, cfg), "YES", false)
	m9AssertColumn(t, columns, "event_data", "text", "NO", false)
	m9AssertColumn(t, columns, "occurred_at", m3ExpectedTimestampType(t, cfg), "NO", false)
	m9AssertColumn(t, columns, "recorded_at", m3ExpectedTimestampType(t, cfg), "NO", true)

	expectedScopeLength := int64(16)
	expectedActionLength := int64(128)
	if cfg.Driver == "mysql" {
		expectedScopeLength = 32
		expectedActionLength = 256
		for _, column := range []string{"id", "account_id", "actor_id", "operation_id"} {
			require.Equal(t, int64(16), columns[column].maximumLength.Int64)
		}
	}
	require.Equal(t, expectedScopeLength, columns["scope"].maximumLength.Int64)
	require.Equal(t, expectedScopeLength, columns["actor_type"].maximumLength.Int64)
	require.Equal(t, expectedActionLength, columns["action"].maximumLength.Int64)
	require.Equal(t, expectedActionLength, columns["request_id"].maximumLength.Int64)
	require.Equal(t, int64(6), m17TimestampPrecision(t, ctx, db, cfg, "occurred_at"))
	require.Equal(t, int64(6), m17TimestampPrecision(t, ctx, db, cfg, "recorded_at"))

	require.True(t, migrationHasExactConstraintColumns(
		t, ctx, db, cfg, m17AuditEventsTable, "PRIMARY KEY", []string{"id"},
	))
	foreignKeys := migrationForeignKeys(t, ctx, db, cfg, m17AuditEventsTable)
	require.Len(t, foreignKeys, 2)
	for _, foreignKey := range []migrationForeignKey{
		{
			localColumns:      []string{"account_id"},
			referencedTable:   m1AccountsTable,
			referencedColumns: []string{"id"},
			updateRule:        m10ExpectedRestrictiveRule(cfg),
			deleteRule:        m10ExpectedRestrictiveRule(cfg),
		},
		{
			localColumns:      []string{"actor_id"},
			referencedTable:   m3UsersTable,
			referencedColumns: []string{"id"},
			updateRule:        m10ExpectedRestrictiveRule(cfg),
			deleteRule:        m10ExpectedRestrictiveRule(cfg),
		},
	} {
		require.True(t, migrationHasExactForeignKey(
			t, ctx, db, cfg, m17AuditEventsTable, foreignKey,
		))
	}

	indexes := migrationIndexes(t, ctx, db, cfg, m17AuditEventsTable)
	require.Len(t, indexes, 6, "M17 needs only its primary key and five approved read indexes")
	for _, indexColumns := range [][]string{
		{"account_id", "recorded_at", "id"},
		{"actor_id", "actor_type", "account_id", "recorded_at", "id"},
		{"account_id", "operation_id", "recorded_at", "id"},
		{"account_id", "request_id", "recorded_at", "id"},
		{"recorded_at", "id"},
	} {
		require.True(t, migrationHasExactIndexColumns(
			t, ctx, db, cfg, m17AuditEventsTable, false, indexColumns,
		))
	}
}

func m17TimestampPrecision(
	t *testing.T,
	ctx context.Context,
	db *sql.DB,
	cfg config.DatabaseConfig,
	column string,
) int64 {
	t.Helper()

	var precision int64
	require.NoError(t, db.QueryRowContext(
		ctx,
		"SELECT datetime_precision FROM information_schema.columns WHERE table_schema = "+
			migrationPlaceholder(cfg, 1)+" AND table_name = "+migrationPlaceholder(cfg, 2)+
			" AND column_name = "+migrationPlaceholder(cfg, 3),
		migrationCurrentSchema(t, ctx, db, cfg),
		m17AuditEventsTable,
		column,
	).Scan(&precision))
	return precision
}

func m17NamedConstraintClause(t *testing.T, source, constraint string) string {
	t.Helper()

	marker := "CONSTRAINT " + constraint
	start := strings.Index(source, marker)
	require.NotEqual(t, -1, start, "required constraint %s is missing", constraint)
	remainder := source[start:]
	end := len(remainder)
	for _, boundary := range []string{"\n    CONSTRAINT ", "\n) ENGINE"} {
		if index := strings.Index(remainder[len(marker):], boundary); index >= 0 {
			candidate := len(marker) + index
			if candidate < end {
				end = candidate
			}
		}
	}
	return remainder[:end]
}

func m17CreatePreexistingParents(
	t *testing.T,
	ctx context.Context,
	db *sql.DB,
	cfg config.DatabaseConfig,
) {
	t.Helper()
	m2InsertAccount(t, ctx, db, cfg, m1AccountID(t, cfg, 1), "m17-preexisting-account")
	m2InsertUser(t, ctx, db, cfg, m3UserID(t, cfg, 1), "m17-preexisting@example.test")
}

func m17AssertNoSeededEvents(t *testing.T, ctx context.Context, db *sql.DB) {
	t.Helper()
	require.Zero(t, m17TableCount(t, ctx, db))
}

type m17AccountSnapshot struct {
	id        string
	slug      string
	status    string
	createdAt time.Time
	updatedAt time.Time
}

func m17SnapshotAccounts(t *testing.T, ctx context.Context, db *sql.DB) []m17AccountSnapshot {
	t.Helper()

	rows, err := db.QueryContext(
		ctx,
		"SELECT id, slug, status, created_at, updated_at FROM accounts ORDER BY slug",
	)
	require.NoError(t, err)
	defer func() { require.NoError(t, rows.Close()) }()

	var snapshot []m17AccountSnapshot
	for rows.Next() {
		var id any
		var record m17AccountSnapshot
		require.NoError(t, rows.Scan(
			&id,
			&record.slug,
			&record.status,
			&record.createdAt,
			&record.updatedAt,
		))
		record.id = m15SnapshotID(id)
		snapshot = append(snapshot, record)
	}
	require.NoError(t, rows.Err())
	return snapshot
}

func m17AssertAccountsSchemaOnly(
	t *testing.T,
	ctx context.Context,
	db *sql.DB,
	cfg config.DatabaseConfig,
) {
	t.Helper()

	columns := migrationColumns(t, ctx, db, cfg, m1AccountsTable)
	require.Len(t, columns, 5, "accounts schema must survive M17 rollback unchanged")
	m9AssertColumn(t, columns, "id", m1ExpectedIDType(t, cfg), "NO", false)
	m9AssertColumn(t, columns, "slug", m1ExpectedVarcharType(t, cfg), "NO", false)
	m9AssertColumn(t, columns, "status", m1ExpectedVarcharType(t, cfg), "NO", false)
	m9AssertColumn(t, columns, "created_at", m1ExpectedTimestampType(t, cfg), "NO", true)
	m9AssertColumn(t, columns, "updated_at", m1ExpectedTimestampType(t, cfg), "NO", true)
	require.True(t, migrationHasExactConstraintColumns(
		t, ctx, db, cfg, m1AccountsTable, "PRIMARY KEY", []string{"id"},
	))
	require.True(t, migrationHasExactConstraintColumns(
		t, ctx, db, cfg, m1AccountsTable, "UNIQUE", []string{"slug"},
	))
}

func m17TableCount(t *testing.T, ctx context.Context, db *sql.DB) int {
	t.Helper()
	var count int
	require.NoError(t, db.QueryRowContext(ctx, "SELECT COUNT(*) FROM audit_events").Scan(&count))
	return count
}

func m17CleanupSchema(t *testing.T, db *sql.DB) {
	t.Helper()

	cleanupContext, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	for _, table := range []string{
		m17AuditEventsTable,
		m17VersionTable,
		m3UsersTable,
		m1AccountsTable,
		m3VersionTable,
		m1MigrationTableName,
	} {
		_, err := db.ExecContext(cleanupContext, "DROP TABLE IF EXISTS "+table)
		require.NoError(t, err)
	}
}
