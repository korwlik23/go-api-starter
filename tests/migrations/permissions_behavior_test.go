package migrations_test

import (
	"bytes"
	"context"
	"database/sql"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/korwlik23/go-api-starter/internal/platform/config"
	"github.com/stretchr/testify/require"
)

func m9AssertPermissionsBehavior(
	t *testing.T,
	ctx context.Context,
	db *sql.DB,
	cfg config.DatabaseConfig,
	seedOffset byte,
) {
	t.Helper()

	createdAt := time.Now().UTC().Truncate(time.Microsecond)
	updatedAt := createdAt.Add(time.Second)
	insertSQL := "INSERT INTO permissions (id, permission_key, module_id, scope, tier, " +
		"delegable, active, created_at, updated_at) VALUES (" +
		migrationPlaceholders(cfg, 9) + ")"

	readID := m9PermissionID(t, cfg, 10+seedOffset)
	_, err := db.ExecContext(
		ctx,
		insertSQL,
		readID,
		"publishing.pages.read.any",
		"publishing",
		"any",
		"default",
		true,
		true,
		createdAt,
		updatedAt,
	)
	require.NoError(t, err)

	_, err = db.ExecContext(
		ctx,
		insertSQL,
		m9PermissionID(t, cfg, 11+seedOffset),
		"localization.translations.update.system",
		"localization",
		"system",
		"core",
		false,
		true,
		createdAt,
		updatedAt,
	)
	require.NoError(t, err)

	m9AssertPermissionDefaults(t, ctx, db, cfg, createdAt, seedOffset)
	m9AssertPermissionBoundaries(t, ctx, db, cfg, insertSQL, createdAt, updatedAt, seedOffset)
	m9AssertMySQLNonStrictBoundary(t, ctx, db, cfg, insertSQL, createdAt, updatedAt, seedOffset)
	m9AssertPermissionUniqueness(t, ctx, db, cfg, insertSQL, readID, createdAt, updatedAt, seedOffset)
	m9AssertInvalidPermissionCatalog(t, ctx, db, cfg, insertSQL, createdAt, updatedAt, seedOffset)
	m9AssertPermissionLifecycle(t, ctx, db, cfg, readID, createdAt, updatedAt)
}

func m9AssertPermissionDefaults(
	t *testing.T,
	ctx context.Context,
	db *sql.DB,
	cfg config.DatabaseConfig,
	createdAt time.Time,
	seedOffset byte,
) {
	t.Helper()

	key := fmt.Sprintf("publishing.pages.update_%d.own", seedOffset)
	_, err := db.ExecContext(
		ctx,
		"INSERT INTO permissions (id, permission_key, module_id, scope, tier) VALUES ("+
			migrationPlaceholders(cfg, 5)+")",
		m9PermissionID(t, cfg, 12+seedOffset),
		key,
		"publishing",
		"own",
		"default",
	)
	require.NoError(t, err)

	var delegable bool
	var active bool
	var storedCreatedAt time.Time
	var storedUpdatedAt time.Time
	require.NoError(t, db.QueryRowContext(
		ctx,
		"SELECT delegable, active, created_at, updated_at FROM permissions "+
			"WHERE permission_key = "+migrationPlaceholder(cfg, 1),
		key,
	).Scan(&delegable, &active, &storedCreatedAt, &storedUpdatedAt))
	require.False(t, delegable, "permissions must default to non-delegable")
	require.False(t, active, "permissions must default inactive")
	require.False(t, storedCreatedAt.IsZero())
	require.False(t, storedUpdatedAt.IsZero())
	require.False(t, storedCreatedAt.Before(createdAt.Add(-time.Minute)))
}

func m9AssertPermissionBoundaries(
	t *testing.T,
	ctx context.Context,
	db *sql.DB,
	cfg config.DatabaseConfig,
	insertSQL string,
	createdAt time.Time,
	updatedAt time.Time,
	seedOffset byte,
) {
	t.Helper()

	moduleID := "m" + strings.Repeat("o", 39)
	permissionKey := moduleID + "." + strings.Repeat("r", 104) + "." +
		strings.Repeat("a", 105) + ".any"
	require.Len(t, moduleID, 40)
	require.Len(t, permissionKey, 255)
	_, err := db.ExecContext(
		ctx,
		insertSQL,
		m9PermissionID(t, cfg, 13+seedOffset),
		permissionKey,
		moduleID,
		"any",
		"optional",
		false,
		false,
		createdAt,
		updatedAt,
	)
	require.NoError(t, err, "maximum canonical catalog identifiers must remain representable")

	overlongCanonicalKey := moduleID + "." + strings.Repeat("r", 104) + "." +
		strings.Repeat("a", 106) + ".any"
	require.Len(t, overlongCanonicalKey, 256)
	_, err = db.ExecContext(
		ctx,
		insertSQL,
		m9PermissionID(t, cfg, 15+seedOffset),
		overlongCanonicalKey,
		moduleID,
		"any",
		"optional",
		false,
		false,
		createdAt,
		updatedAt,
	)
	require.Error(t, err, "a canonical permission key above the logical limit must be rejected")
}

func m9AssertMySQLNonStrictBoundary(
	t *testing.T,
	ctx context.Context,
	db *sql.DB,
	cfg config.DatabaseConfig,
	insertSQL string,
	createdAt time.Time,
	updatedAt time.Time,
	seedOffset byte,
) {
	t.Helper()
	if cfg.Driver != "mysql" {
		return
	}

	connection, err := db.Conn(ctx)
	require.NoError(t, err)
	var previousSQLMode string
	require.NoError(t, connection.QueryRowContext(ctx, "SELECT @@SESSION.sql_mode").Scan(
		&previousSQLMode,
	))
	require.NoError(t, m9SetSQLMode(ctx, connection, ""))
	defer func() {
		restoreContext, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		require.NoError(t, m9SetSQLMode(restoreContext, connection, previousSQLMode))
		require.NoError(t, connection.Close())
	}()

	moduleID := "n" + strings.Repeat("o", 39)
	validPrefix := moduleID + "." + strings.Repeat("r", 103) + "." +
		strings.Repeat("a", 106) + ".any"
	require.Len(t, validPrefix, 255)
	_, err = connection.ExecContext(
		ctx,
		insertSQL,
		m9PermissionID(t, cfg, 14+seedOffset),
		validPrefix+"x",
		moduleID,
		"any",
		"optional",
		false,
		false,
		createdAt,
		updatedAt,
	)
	require.Error(t, err, "non-strict mode must not truncate a 256-byte key into a valid key")
}

func m9SetSQLMode(ctx context.Context, connection *sql.Conn, mode string) error {
	_, err := connection.ExecContext(ctx, "SET SESSION sql_mode = ?", mode)
	return err
}

func m9AssertPermissionUniqueness(
	t *testing.T,
	ctx context.Context,
	db *sql.DB,
	cfg config.DatabaseConfig,
	insertSQL string,
	readID any,
	createdAt time.Time,
	updatedAt time.Time,
	seedOffset byte,
) {
	t.Helper()

	_, err := db.ExecContext(
		ctx,
		insertSQL,
		m9PermissionID(t, cfg, 20+seedOffset),
		"publishing.pages.read.any",
		"publishing",
		"any",
		"default",
		true,
		true,
		createdAt,
		updatedAt,
	)
	require.Error(t, err, "permission keys must be globally unique")

	_, err = db.ExecContext(
		ctx,
		insertSQL,
		readID,
		"publishing.pages.archive.any",
		"publishing",
		"any",
		"default",
		true,
		true,
		createdAt,
		updatedAt,
	)
	require.Error(t, err, "permission IDs must be globally unique")

	var foldedMatches int
	require.NoError(t, db.QueryRowContext(
		ctx,
		"SELECT COUNT(*) FROM permissions WHERE permission_key = "+
			migrationPlaceholder(cfg, 1),
		"PUBLISHING.PAGES.READ.ANY",
	).Scan(&foldedMatches))
	require.Zero(t, foldedMatches, "permission key lookup must be case-sensitive")
}

func m9AssertInvalidPermissionCatalog(
	t *testing.T,
	ctx context.Context,
	db *sql.DB,
	cfg config.DatabaseConfig,
	insertSQL string,
	createdAt time.Time,
	updatedAt time.Time,
	seedOffset byte,
) {
	t.Helper()

	invalidKeys := []string{
		"Publishing.pages.read.any",
		"publishing.pages.read",
		"publishing.pages.read.any.extra",
		"publishing..read.any",
		"publishing.pages-read.any",
		"publishing.pages.Read.any",
		"1publishing.pages.read.any",
		"publishing.pages.read.all",
		"publishing.pages.re\u00e4d.any",
	}
	for index, key := range invalidKeys {
		_, err := db.ExecContext(
			ctx,
			insertSQL,
			m9PermissionID(t, cfg, byte(30+index)+seedOffset),
			key,
			"publishing",
			"any",
			"default",
			false,
			false,
			createdAt,
			updatedAt,
		)
		require.Error(t, err, "invalid canonical permission key %d must be rejected", index)
	}

	invalidMetadata := []struct {
		name     string
		key      string
		moduleID string
		scope    string
		tier     string
	}{
		{name: "module mismatch", key: "publishing.pages.module_mismatch.any", moduleID: "media", scope: "any", tier: "default"},
		{name: "scope mismatch", key: "publishing.pages.scope_mismatch.own", moduleID: "publishing", scope: "any", tier: "default"},
		{name: "unknown scope", key: "publishing.pages.unknown_scope.any", moduleID: "publishing", scope: "all", tier: "default"},
		{name: "uppercase scope", key: "publishing.pages.uppercase_scope.any", moduleID: "publishing", scope: "ANY", tier: "default"},
		{name: "unknown tier", key: "publishing.pages.unknown_tier.any", moduleID: "publishing", scope: "any", tier: "premium"},
		{name: "uppercase tier", key: "publishing.pages.uppercase_tier.any", moduleID: "publishing", scope: "any", tier: "CORE"},
		{name: "module too long", key: strings.Repeat("a", 41) + ".pages.read.any", moduleID: strings.Repeat("a", 41), scope: "any", tier: "default"},
	}
	for index, metadata := range invalidMetadata {
		_, err := db.ExecContext(
			ctx,
			insertSQL,
			m9PermissionID(t, cfg, byte(45+index)+seedOffset),
			metadata.key,
			metadata.moduleID,
			metadata.scope,
			metadata.tier,
			false,
			false,
			createdAt,
			updatedAt,
		)
		require.Error(t, err, metadata.name)
	}

	for index, booleans := range [][2]int{{2, 0}, {0, 2}} {
		_, err := db.ExecContext(
			ctx,
			insertSQL,
			m9PermissionID(t, cfg, byte(55+index)+seedOffset),
			fmt.Sprintf("publishing.pages.boolean_%d.any", index),
			"publishing",
			"any",
			"default",
			booleans[0],
			booleans[1],
			createdAt,
			updatedAt,
		)
		require.Error(t, err, "boolean value outside 0 or 1 must be rejected")
	}

	_, err := db.ExecContext(
		ctx,
		insertSQL,
		m9PermissionID(t, cfg, 57+seedOffset),
		"publishing.pages.invalid_time.any",
		"publishing",
		"any",
		"default",
		false,
		false,
		createdAt,
		createdAt.Add(-time.Second),
	)
	require.Error(t, err, "updated_at cannot precede created_at")
}

func m9AssertPermissionLifecycle(
	t *testing.T,
	ctx context.Context,
	db *sql.DB,
	cfg config.DatabaseConfig,
	permissionID any,
	createdAt time.Time,
	updatedAt time.Time,
) {
	t.Helper()

	result, err := db.ExecContext(
		ctx,
		"UPDATE permissions SET active = "+migrationPlaceholder(cfg, 1)+
			", updated_at = "+migrationPlaceholder(cfg, 2)+
			" WHERE id = "+migrationPlaceholder(cfg, 3)+
			" AND active = "+migrationPlaceholder(cfg, 4),
		false,
		updatedAt.Add(time.Second),
		permissionID,
		true,
	)
	require.NoError(t, err)
	affected, err := result.RowsAffected()
	require.NoError(t, err)
	require.Equal(t, int64(1), affected)

	var active bool
	var storedUpdatedAt time.Time
	require.NoError(t, db.QueryRowContext(
		ctx,
		"SELECT active, updated_at FROM permissions WHERE id = "+
			migrationPlaceholder(cfg, 1),
		permissionID,
	).Scan(&active, &storedUpdatedAt))
	require.False(t, active)
	require.WithinDuration(t, updatedAt.Add(time.Second), storedUpdatedAt, time.Microsecond)
	require.False(t, storedUpdatedAt.Before(createdAt))
}

func m9PermissionID(t *testing.T, cfg config.DatabaseConfig, seed byte) any {
	t.Helper()

	if cfg.Driver == "postgres" {
		return fmt.Sprintf("%08x-0000-4000-8000-%012x", seed, seed)
	}
	if cfg.Driver == "mysql" {
		return bytes.Repeat([]byte{seed}, 16)
	}
	t.Fatalf("unsupported M9 database profile")
	return nil
}
