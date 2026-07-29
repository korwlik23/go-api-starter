package gorm

import (
	"context"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	authorizationdomain "github.com/korwlik23/go-api-starter/internal/modules/authorization/domain"
	"github.com/korwlik23/go-api-starter/internal/modules/authorization/ports"
	"github.com/stretchr/testify/require"
)

func TestRoleRepositoryScopesLookupAndCASMutationByAccountAndRoleID(
	t *testing.T,
) {
	db, mock := permissionRepositoryDatabase(t)
	repository, err := NewRoleRepository(db)
	require.NoError(t, err)
	accountID := "018f1f64-7b2a-7c10-8bf2-3c40d5f67581"
	roleID := "018f1f64-7b2a-7c10-8bf2-3c40d5f67582"
	mock.ExpectQuery(`SELECT .* FROM "roles"`).
		WithArgs(roleID, accountID, "account", 1).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "owner_type", "account_id", "name", "version",
		}).AddRow(roleID, "account", accountID, "Editors", 3))

	role, err := repository.FindAccountRole(
		context.Background(),
		accountID,
		roleID,
	)
	require.NoError(t, err)
	require.Equal(t, authorizationdomain.RoleOwnerAccount, role.OwnerType)
	require.Equal(t, int64(3), role.Version)

	mock.ExpectBegin()
	mock.ExpectExec(`UPDATE "roles"`).
		WithArgs("Reviewers", int64(4), roleID, accountID, "account", int64(3)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	version, err := repository.UpdateAccountRoleName(
		context.Background(),
		accountID,
		roleID,
		3,
		"Reviewers",
	)
	require.NoError(t, err)
	require.Equal(t, int64(4), version)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestRoleRepositoryReturnsConflictWithoutSideEffectsOnStaleVersion(
	t *testing.T,
) {
	db, mock := permissionRepositoryDatabase(t)
	repository, err := NewRoleRepository(db)
	require.NoError(t, err)
	accountID := "018f1f64-7b2a-7c10-8bf2-3c40d5f67581"
	roleID := "018f1f64-7b2a-7c10-8bf2-3c40d5f67582"
	mock.ExpectBegin()
	mock.ExpectExec(`UPDATE "roles"`).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectCommit()

	_, err = repository.UpdateAccountRoleName(
		context.Background(),
		accountID,
		roleID,
		3,
		"Reviewers",
	)

	require.ErrorIs(t, err, ports.ErrRoleConflict)
}
