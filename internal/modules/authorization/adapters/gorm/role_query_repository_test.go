package gorm

import (
	"context"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/korwlik23/go-api-starter/internal/modules/authorization/ports"
	"github.com/stretchr/testify/require"
)

func TestRoleQueryRepositoryListsBoundedRoles(t *testing.T) {
	db, mock := permissionRepositoryDatabase(t)
	repository, err := NewRoleQueryRepository(db)
	require.NoError(t, err)

	mock.ExpectQuery(`SELECT count\(\*\) FROM "roles"`).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(2))
	mock.ExpectQuery(`SELECT .* FROM "roles".*ORDER BY`).
		WithArgs(25).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "owner_type", "account_id", "name", "version", "created_at", "updated_at",
		}).
			AddRow(
				"018f1f64-7b2a-7c10-8bf2-3c40d5f67581",
				"account",
				"018f1f64-7b2a-7c10-8bf2-3c40d5f67582",
				"Editors",
				3,
				time.Date(2026, time.August, 1, 0, 0, 0, 0, time.UTC),
				time.Date(2026, time.August, 1, 0, 0, 0, 0, time.UTC),
			))

	page, err := repository.ListRoles(context.Background(), ports.RoleListFilter{
		RolePageRequest: ports.RolePageRequest{Page: 1, PerPage: 25},
	})
	require.NoError(t, err)
	require.Len(t, page.Items, 1)
	require.Equal(t, "Editors", page.Items[0].Name)
	require.Equal(t, int64(3), page.Items[0].Version)
	require.Equal(t, 2, page.Page.TotalItems)
	require.Equal(t, 1, page.Page.TotalPages)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestRoleQueryRepositoryListsRolePermissions(t *testing.T) {
	db, mock := permissionRepositoryDatabase(t)
	repository, err := NewRoleQueryRepository(db)
	require.NoError(t, err)
	roleID := "018f1f64-7b2a-7c10-8bf2-3c40d5f67581"

	mock.ExpectQuery(`SELECT id, version FROM roles WHERE id = \$1`).
		WithArgs(roleID).
		WillReturnRows(sqlmock.NewRows([]string{"id", "version"}).AddRow(roleID, 3))
	mock.ExpectQuery(`SELECT permissions\.permission_key, permissions\.active, permissions\.delegable, permissions\.tier`).
		WithArgs(roleID, 256).
		WillReturnRows(sqlmock.NewRows([]string{
			"permission_key", "active", "delegable", "tier",
		}).AddRow("authorization.roles.read.system", true, false, "core"))

	items, err := repository.ListRolePermissions(context.Background(), roleID)
	require.NoError(t, err)
	require.Equal(t, ports.RolePermissionList{RoleID: roleID, RoleVersion: 3, Items: []ports.RolePermissionView{{
		PermissionKey: "authorization.roles.read.system",
		Active:        true,
		Delegable:     false,
		Tier:          "core",
	}}}, items)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestRoleQueryRepositoryListsAssignmentsAcrossScopes(t *testing.T) {
	db, mock := permissionRepositoryDatabase(t)
	repository, err := NewRoleQueryRepository(db)
	require.NoError(t, err)
	roleID := "018f1f64-7b2a-7c10-8bf2-3c40d5f67581"

	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM \(`).
		WithArgs(roleID, roleID).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	mock.ExpectQuery(`SELECT assignment_id, role_id, scope, account_id, user_id, created_at FROM \(`).
		WithArgs(roleID, roleID, 25, 0).
		WillReturnRows(sqlmock.NewRows([]string{
			"assignment_id", "role_id", "scope", "account_id", "user_id", "created_at",
		}).AddRow(
			"018f1f64-7b2a-7c10-8bf2-3c40d5f67582",
			roleID,
			"account",
			"018f1f64-7b2a-7c10-8bf2-3c40d5f67583",
			"018f1f64-7b2a-7c10-8bf2-3c40d5f67584",
			time.Date(2026, time.August, 1, 0, 0, 0, 0, time.UTC),
		))

	page, err := repository.ListRoleAssignments(
		context.Background(),
		roleID,
		ports.RolePageRequest{Page: 1, PerPage: 25},
	)
	require.NoError(t, err)
	require.Len(t, page.Items, 1)
	require.Equal(t, "account", page.Items[0].Scope)
	require.Equal(t, "018f1f64-7b2a-7c10-8bf2-3c40d5f67584", page.Items[0].UserID)
	require.Equal(t, 1, page.Page.TotalItems)
	require.NoError(t, mock.ExpectationsWereMet())
}
