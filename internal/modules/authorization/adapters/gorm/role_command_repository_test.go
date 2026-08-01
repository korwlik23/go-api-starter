package gorm

import (
	"context"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	authorizationdomain "github.com/korwlik23/go-api-starter/internal/modules/authorization/domain"
	"github.com/korwlik23/go-api-starter/internal/modules/authorization/ports"
	"github.com/stretchr/testify/require"
	gormdb "gorm.io/gorm"
)

func TestRoleCommandRepositoryCreatesAccountRole(t *testing.T) {
	db, mock := permissionRepositoryDatabase(t)
	repository, err := NewRoleCommandRepository(db)
	require.NoError(t, err)
	accountID := "018f1f64-7b2a-7c10-8bf2-3c40d5f67581"

	mock.ExpectExec(`INSERT INTO roles`).
		WithArgs(
			sqlmock.AnyArg(), "account", accountID, "Editors", int64(1),
			sqlmock.AnyArg(), sqlmock.AnyArg(),
		).
		WillReturnResult(sqlmock.NewResult(0, 1))

	role, err := repository.CreateAccountRole(context.Background(), accountID, "Editors")
	require.NoError(t, err)
	require.Equal(t, authorizationdomain.RoleOwnerAccount, role.OwnerType)
	require.Equal(t, accountID, role.AccountID)
	require.Equal(t, "Editors", role.Name)
	require.Equal(t, int64(1), role.Version)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestRoleCommandRepositoryUpdatesAccountRoleWithCAS(t *testing.T) {
	db, mock := permissionRepositoryDatabase(t)
	repository, err := NewRoleCommandRepository(db)
	require.NoError(t, err)
	accountID := "018f1f64-7b2a-7c10-8bf2-3c40d5f67581"
	roleID := "018f1f64-7b2a-7c10-8bf2-3c40d5f67582"

	mock.ExpectQuery(`SELECT .* FROM "roles"`).
		WithArgs(roleID, accountID, "account", 1).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "owner_type", "account_id", "name", "version", "created_at", "updated_at",
		}).AddRow(
			roleID, "account", accountID, "Editors", 3,
			time.Date(2026, time.August, 1, 0, 0, 0, 0, time.UTC),
			time.Date(2026, time.August, 1, 0, 0, 0, 0, time.UTC),
		))
	mock.ExpectExec(`UPDATE roles`).
		WithArgs(
			"Reviewers", int64(4), sqlmock.AnyArg(),
			roleID, accountID, "account", int64(3),
		).
		WillReturnResult(sqlmock.NewResult(0, 1))

	role, err := repository.UpdateAccountRoleName(
		context.Background(), accountID, roleID, 3, "Reviewers",
	)
	require.NoError(t, err)
	require.Equal(t, roleID, role.ID)
	require.Equal(t, accountID, role.AccountID)
	require.Equal(t, "Reviewers", role.Name)
	require.Equal(t, int64(4), role.Version)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestRoleCommandRepositoryReturnsConflictForStaleVersion(t *testing.T) {
	db, mock := permissionRepositoryDatabase(t)
	repository, err := NewRoleCommandRepository(db)
	require.NoError(t, err)
	mock.ExpectQuery(`SELECT .* FROM "roles"`).
		WithArgs(
			"018f1f64-7b2a-7c10-8bf2-3c40d5f67582",
			"018f1f64-7b2a-7c10-8bf2-3c40d5f67581",
			"account",
			1,
		).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "owner_type", "account_id", "name", "version", "created_at", "updated_at",
		}).AddRow(
			"018f1f64-7b2a-7c10-8bf2-3c40d5f67582",
			"account",
			"018f1f64-7b2a-7c10-8bf2-3c40d5f67581",
			"Editors",
			3,
			time.Date(2026, time.August, 1, 0, 0, 0, 0, time.UTC),
			time.Date(2026, time.August, 1, 0, 0, 0, 0, time.UTC),
		))
	mock.ExpectExec(`UPDATE roles`).
		WillReturnResult(sqlmock.NewResult(0, 0))

	_, err = repository.UpdateAccountRoleName(
		context.Background(),
		"018f1f64-7b2a-7c10-8bf2-3c40d5f67581",
		"018f1f64-7b2a-7c10-8bf2-3c40d5f67582",
		3,
		"Reviewers",
	)
	require.ErrorIs(t, err, ports.ErrRoleConflict)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestRoleCommandRepositoryUsesTransactionConnectionProvider(t *testing.T) {
	db, mock := permissionRepositoryDatabase(t)
	provider := &roleCommandConnectionProviderFake{db: db}
	repository, err := NewTransactionalRoleCommandRepository(provider, "postgres")
	require.NoError(t, err)
	accountID := "018f1f64-7b2a-7c10-8bf2-3c40d5f67581"

	mock.ExpectExec(`INSERT INTO roles`).
		WithArgs(
			sqlmock.AnyArg(), "account", accountID, "Editors", int64(1),
			sqlmock.AnyArg(), sqlmock.AnyArg(),
		).
		WillReturnResult(sqlmock.NewResult(0, 1))

	_, err = repository.CreateAccountRole(context.Background(), accountID, "Editors")
	require.NoError(t, err)
	require.True(t, provider.called)
	require.NoError(t, mock.ExpectationsWereMet())
}

type roleCommandConnectionProviderFake struct {
	db     *gormdb.DB
	called bool
}

func (provider *roleCommandConnectionProviderFake) Connection(ctx context.Context) (*gormdb.DB, error) {
	provider.called = true
	return provider.db.WithContext(ctx), nil
}
