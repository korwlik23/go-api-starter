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

type m11PermissionFixtures struct {
	activeDelegable     any
	inactiveDelegable   any
	activeNonDelegable  any
	transitionedDormant any
	initiallyDormant    any
	unreferencedControl any
}

func m11AssertRolePermissionBehavior(
	t *testing.T,
	ctx context.Context,
	db *sql.DB,
	cfg config.DatabaseConfig,
	seedOffset byte,
) {
	t.Helper()

	accountID := m1AccountID(t, cfg, 1+seedOffset)
	m2InsertAccount(t, ctx, db, cfg, accountID, fmt.Sprintf("mapping-account-%d", seedOffset))
	roleOne := m10RoleID(t, cfg, 10+seedOffset)
	roleTwo := m10RoleID(t, cfg, 11+seedOffset)
	roleControl := m10RoleID(t, cfg, 12+seedOffset)
	m11InsertAccountRole(t, ctx, db, cfg, roleOne, accountID, "First dynamic role")
	m11InsertAccountRole(t, ctx, db, cfg, roleTwo, accountID, "Second dynamic role")
	m11InsertAccountRole(t, ctx, db, cfg, roleControl, accountID, "Unmapped control role")

	permissions := m11PermissionFixtures{
		activeDelegable:     m9PermissionID(t, cfg, 20+seedOffset),
		inactiveDelegable:   m9PermissionID(t, cfg, 21+seedOffset),
		activeNonDelegable:  m9PermissionID(t, cfg, 22+seedOffset),
		transitionedDormant: m9PermissionID(t, cfg, 23+seedOffset),
		initiallyDormant:    m9PermissionID(t, cfg, 24+seedOffset),
		unreferencedControl: m9PermissionID(t, cfg, 25+seedOffset),
	}
	m11InsertPermission(
		t, ctx, db, cfg, permissions.activeDelegable,
		fmt.Sprintf("publishing.pages.read_%d.any", seedOffset), "any", true, true,
	)
	m11InsertPermission(
		t, ctx, db, cfg, permissions.inactiveDelegable,
		fmt.Sprintf("publishing.pages.update_%d.own", seedOffset), "own", true, false,
	)
	m11InsertPermission(
		t, ctx, db, cfg, permissions.activeNonDelegable,
		fmt.Sprintf("publishing.pages.archive_%d.any", seedOffset), "any", false, true,
	)
	m11InsertPermission(
		t, ctx, db, cfg, permissions.transitionedDormant,
		fmt.Sprintf("publishing.pages.publish_%d.own", seedOffset), "own", true, true,
	)
	m11InsertPermission(
		t, ctx, db, cfg, permissions.initiallyDormant,
		fmt.Sprintf("publishing.pages.restore_%d.own", seedOffset), "own", false, false,
	)
	m11InsertPermission(
		t, ctx, db, cfg, permissions.unreferencedControl,
		fmt.Sprintf("publishing.pages.delete_%d.any", seedOffset), "any", false, false,
	)

	insertSQL := "INSERT INTO role_permissions (role_id, permission_id) VALUES (" +
		migrationPlaceholders(cfg, 2) + ")"
	for _, permissionID := range []any{
		permissions.activeDelegable,
		permissions.inactiveDelegable,
		permissions.activeNonDelegable,
		permissions.transitionedDormant,
		permissions.initiallyDormant,
	} {
		_, err := db.ExecContext(ctx, insertSQL, roleOne, permissionID)
		require.NoError(t, err, "permission catalog flags must not alter mapping persistence")
	}
	_, err := db.ExecContext(ctx, insertSQL, roleTwo, permissions.activeDelegable)
	require.NoError(t, err, "one permission may belong to multiple dynamic roles")

	m11AssertMappingIdentityAndFKs(
		t, ctx, db, cfg, insertSQL, roleOne, permissions.activeDelegable, seedOffset,
	)
	m11AssertMappingTimestamp(t, ctx, db, cfg, roleOne, permissions.activeDelegable)
	m11AssertMappedPermissionTransition(
		t, ctx, db, cfg, roleOne, permissions.transitionedDormant,
	)
	m11AssertParentIDUpdatesRestricted(
		t, ctx, db, cfg, roleOne, permissions.activeDelegable, seedOffset,
	)
	m11AssertExplicitDetach(
		t, ctx, db, cfg, insertSQL, roleOne, permissions.activeNonDelegable,
	)
	m11AssertMappingDeletePolicies(
		t,
		ctx,
		db,
		cfg,
		roleOne,
		roleTwo,
		roleControl,
		permissions,
	)
}

func m11AssertMappingIdentityAndFKs(
	t *testing.T,
	ctx context.Context,
	db *sql.DB,
	cfg config.DatabaseConfig,
	insertSQL string,
	roleID any,
	permissionID any,
	seedOffset byte,
) {
	t.Helper()

	_, err := db.ExecContext(ctx, insertSQL, roleID, permissionID)
	require.Error(t, err, "a role-permission pair must be unique")
	_, err = db.ExecContext(
		ctx,
		insertSQL,
		m10RoleID(t, cfg, 90+seedOffset),
		permissionID,
	)
	require.Error(t, err, "an unknown role must be rejected")
	_, err = db.ExecContext(
		ctx,
		insertSQL,
		roleID,
		m9PermissionID(t, cfg, 90+seedOffset),
	)
	require.Error(t, err, "an unknown permission must be rejected")

	_, err = db.ExecContext(
		ctx,
		"INSERT INTO role_permissions (permission_id) VALUES ("+
			migrationPlaceholder(cfg, 1)+")",
		permissionID,
	)
	require.Error(t, err, "role_id must be explicit")
	_, err = db.ExecContext(
		ctx,
		"INSERT INTO role_permissions (role_id) VALUES ("+
			migrationPlaceholder(cfg, 1)+")",
		roleID,
	)
	require.Error(t, err, "permission_id must be explicit")
}

func m11AssertMappingTimestamp(
	t *testing.T,
	ctx context.Context,
	db *sql.DB,
	cfg config.DatabaseConfig,
	roleID any,
	permissionID any,
) {
	t.Helper()

	var createdAt time.Time
	require.NoError(t, db.QueryRowContext(
		ctx,
		"SELECT created_at FROM role_permissions WHERE role_id = "+
			migrationPlaceholder(cfg, 1)+" AND permission_id = "+
			migrationPlaceholder(cfg, 2),
		roleID,
		permissionID,
	).Scan(&createdAt))
	require.False(t, createdAt.IsZero())
}

func m11AssertMappedPermissionTransition(
	t *testing.T,
	ctx context.Context,
	db *sql.DB,
	cfg config.DatabaseConfig,
	roleID any,
	permissionID any,
) {
	t.Helper()

	var previousUpdatedAt time.Time
	require.NoError(t, db.QueryRowContext(
		ctx,
		"SELECT updated_at FROM permissions WHERE id = "+migrationPlaceholder(cfg, 1),
		permissionID,
	).Scan(&previousUpdatedAt))
	transitionedAt := previousUpdatedAt.Add(time.Second)
	_, err := db.ExecContext(
		ctx,
		"UPDATE permissions SET active = "+migrationPlaceholder(cfg, 1)+
			", delegable = "+migrationPlaceholder(cfg, 2)+
			", updated_at = "+migrationPlaceholder(cfg, 3)+
			" WHERE id = "+migrationPlaceholder(cfg, 4),
		false,
		false,
		transitionedAt,
		permissionID,
	)
	require.NoError(t, err)

	var active bool
	var delegable bool
	var mappingCount int
	require.NoError(t, db.QueryRowContext(
		ctx,
		"SELECT active, delegable FROM permissions WHERE id = "+
			migrationPlaceholder(cfg, 1),
		permissionID,
	).Scan(&active, &delegable))
	require.False(t, active)
	require.False(t, delegable)
	require.NoError(t, db.QueryRowContext(
		ctx,
		"SELECT COUNT(*) FROM role_permissions WHERE role_id = "+
			migrationPlaceholder(cfg, 1)+" AND permission_id = "+
			migrationPlaceholder(cfg, 2),
		roleID,
		permissionID,
	).Scan(&mappingCount))
	require.Equal(t, 1, mappingCount, "catalog state changes must preserve dormant mappings")
}

func m11AssertParentIDUpdatesRestricted(
	t *testing.T,
	ctx context.Context,
	db *sql.DB,
	cfg config.DatabaseConfig,
	roleID any,
	permissionID any,
	seedOffset byte,
) {
	t.Helper()

	_, err := db.ExecContext(
		ctx,
		"UPDATE roles SET id = "+migrationPlaceholder(cfg, 1)+
			" WHERE id = "+migrationPlaceholder(cfg, 2),
		m10RoleID(t, cfg, 13+seedOffset),
		roleID,
	)
	require.Error(t, err, "a mapped role ID must remain immutable")
	_, err = db.ExecContext(
		ctx,
		"UPDATE permissions SET id = "+migrationPlaceholder(cfg, 1)+
			" WHERE id = "+migrationPlaceholder(cfg, 2),
		m9PermissionID(t, cfg, 26+seedOffset),
		permissionID,
	)
	require.Error(t, err, "a mapped permission ID must remain immutable")

	var mappingCount int
	require.NoError(t, db.QueryRowContext(
		ctx,
		"SELECT COUNT(*) FROM role_permissions WHERE role_id = "+
			migrationPlaceholder(cfg, 1)+" AND permission_id = "+
			migrationPlaceholder(cfg, 2),
		roleID,
		permissionID,
	).Scan(&mappingCount))
	require.Equal(t, 1, mappingCount, "failed parent updates must preserve the exact mapping")
}

func m11InsertAccountRole(
	t *testing.T,
	ctx context.Context,
	db *sql.DB,
	cfg config.DatabaseConfig,
	roleID any,
	accountID any,
	name string,
) {
	t.Helper()

	_, err := db.ExecContext(
		ctx,
		"INSERT INTO roles (id, owner_type, account_id, name) VALUES ("+
			migrationPlaceholders(cfg, 4)+")",
		roleID,
		"account",
		accountID,
		name,
	)
	require.NoError(t, err)
}

func m11InsertPermission(
	t *testing.T,
	ctx context.Context,
	db *sql.DB,
	cfg config.DatabaseConfig,
	permissionID any,
	permissionKey string,
	scope string,
	delegable bool,
	active bool,
) {
	t.Helper()

	_, err := db.ExecContext(
		ctx,
		"INSERT INTO permissions (id, permission_key, module_id, scope, tier, delegable, active) "+
			"VALUES ("+migrationPlaceholders(cfg, 7)+")",
		permissionID,
		permissionKey,
		"publishing",
		scope,
		"default",
		delegable,
		active,
	)
	require.NoError(t, err)
}
