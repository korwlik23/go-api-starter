package migrations_test

import (
	"context"
	"database/sql"
	"testing"

	"github.com/korwlik23/go-api-starter/internal/platform/config"
	"github.com/stretchr/testify/require"
)

func m11AssertExplicitDetach(
	t *testing.T,
	ctx context.Context,
	db *sql.DB,
	cfg config.DatabaseConfig,
	insertSQL string,
	roleID any,
	permissionID any,
) {
	t.Helper()

	result, err := db.ExecContext(
		ctx,
		"DELETE FROM role_permissions WHERE role_id = "+
			migrationPlaceholder(cfg, 1)+" AND permission_id = "+
			migrationPlaceholder(cfg, 2),
		roleID,
		permissionID,
	)
	require.NoError(t, err)
	affected, err := result.RowsAffected()
	require.NoError(t, err)
	require.Equal(t, int64(1), affected)

	parents := []struct {
		table string
		id    any
	}{
		{table: "roles", id: roleID},
		{table: "permissions", id: permissionID},
	}
	for _, parent := range parents {
		var parentCount int
		require.NoError(t, db.QueryRowContext(
			ctx,
			"SELECT COUNT(*) FROM "+parent.table+" WHERE id = "+
				migrationPlaceholder(cfg, 1),
			parent.id,
		).Scan(&parentCount))
		require.Equal(
			t,
			1,
			parentCount,
			"detaching a mapping must preserve its %s parent",
			parent.table,
		)
	}

	_, err = db.ExecContext(ctx, insertSQL, roleID, permissionID)
	require.NoError(t, err, "an explicitly detached pair must be recreatable")
	m11AssertMappingTimestamp(t, ctx, db, cfg, roleID, permissionID)
}

func m11AssertMappingDeletePolicies(
	t *testing.T,
	ctx context.Context,
	db *sql.DB,
	cfg config.DatabaseConfig,
	roleOne any,
	roleTwo any,
	roleControl any,
	permissions m11PermissionFixtures,
) {
	t.Helper()

	mappedPermissions := []struct {
		id      any
		message string
	}{
		{id: permissions.activeDelegable, message: "an active delegable mapping"},
		{id: permissions.inactiveDelegable, message: "an inactive delegable mapping"},
		{id: permissions.activeNonDelegable, message: "an active non-delegable mapping"},
		{id: permissions.transitionedDormant, message: "a transitioned dormant mapping"},
		{id: permissions.initiallyDormant, message: "an initially dormant mapping"},
	}
	for _, permission := range mappedPermissions {
		_, err := db.ExecContext(
			ctx,
			"DELETE FROM permissions WHERE id = "+migrationPlaceholder(cfg, 1),
			permission.id,
		)
		require.Error(t, err, "%s must restrict permission deletion", permission.message)

		var mappingCount int
		require.NoError(t, db.QueryRowContext(
			ctx,
			"SELECT COUNT(*) FROM role_permissions WHERE permission_id = "+
				migrationPlaceholder(cfg, 1),
			permission.id,
		).Scan(&mappingCount))
		require.Positive(t, mappingCount, "failed permission deletion must preserve mappings")
	}

	_, err := db.ExecContext(
		ctx,
		"DELETE FROM permissions WHERE id = "+migrationPlaceholder(cfg, 1),
		permissions.unreferencedControl,
	)
	require.NoError(t, err, "the unreferenced permission control must remain deletable")

	var mappingsBeforeControlDelete int
	require.NoError(t, db.QueryRowContext(
		ctx,
		"SELECT COUNT(*) FROM role_permissions",
	).Scan(&mappingsBeforeControlDelete))
	_, err = db.ExecContext(
		ctx,
		"DELETE FROM roles WHERE id = "+migrationPlaceholder(cfg, 1),
		roleControl,
	)
	require.NoError(t, err, "an unmapped role control must remain deletable")
	var mappingsAfterControlDelete int
	require.NoError(t, db.QueryRowContext(
		ctx,
		"SELECT COUNT(*) FROM role_permissions",
	).Scan(&mappingsAfterControlDelete))
	require.Equal(
		t,
		mappingsBeforeControlDelete,
		mappingsAfterControlDelete,
		"deleting an unmapped role must not affect another role's mappings",
	)

	_, err = db.ExecContext(
		ctx,
		"DELETE FROM roles WHERE id = "+migrationPlaceholder(cfg, 1),
		roleOne,
	)
	require.NoError(t, err)
	var removedMappings int
	require.NoError(t, db.QueryRowContext(
		ctx,
		"SELECT COUNT(*) FROM role_permissions WHERE role_id = "+
			migrationPlaceholder(cfg, 1),
		roleOne,
	).Scan(&removedMappings))
	require.Zero(t, removedMappings, "deleting a role must cascade only its mappings")

	var retainedMappings int
	require.NoError(t, db.QueryRowContext(
		ctx,
		"SELECT COUNT(*) FROM role_permissions WHERE role_id = "+
			migrationPlaceholder(cfg, 1)+" AND permission_id = "+
			migrationPlaceholder(cfg, 2),
		roleTwo,
		permissions.activeDelegable,
	).Scan(&retainedMappings))
	require.Equal(t, 1, retainedMappings, "another role's mapping must remain intact")

	var retainedPermissions int
	require.NoError(t, db.QueryRowContext(
		ctx,
		"SELECT COUNT(*) FROM permissions WHERE id IN ("+
			migrationPlaceholders(cfg, 5)+")",
		permissions.activeDelegable,
		permissions.inactiveDelegable,
		permissions.activeNonDelegable,
		permissions.transitionedDormant,
		permissions.initiallyDormant,
	).Scan(&retainedPermissions))
	require.Equal(t, 5, retainedPermissions, "role deletion must never cascade permission catalog rows")

	for _, permissionID := range []any{
		permissions.inactiveDelegable,
		permissions.activeNonDelegable,
		permissions.transitionedDormant,
		permissions.initiallyDormant,
	} {
		_, err = db.ExecContext(
			ctx,
			"DELETE FROM permissions WHERE id = "+migrationPlaceholder(cfg, 1),
			permissionID,
		)
		require.NoError(t, err, "a permission becomes deletable after its final mapping is removed")
	}
}
