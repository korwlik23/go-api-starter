package gorm

import (
	"context"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	authorizationdomain "github.com/korwlik23/go-api-starter/internal/modules/authorization/domain"
	"github.com/korwlik23/go-api-starter/internal/modules/authorization/ports"
	"github.com/korwlik23/go-api-starter/internal/platform/databaseid"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	gormdb "gorm.io/gorm"
)

func TestPermissionRepositoryReadsActiveRoleUnionWithCatalogBinding(
	t *testing.T,
) {
	db, mock := permissionRepositoryDatabase(t)
	repository, err := NewPermissionRepository(db)
	require.NoError(t, err)
	userID := "018f1f64-7b2a-7c10-8bf2-3c40d5f67586"
	accountID := "018f1f64-7b2a-7c10-8bf2-3c40d5f67581"
	mock.ExpectQuery(`SELECT[\s\S]+FROM memberships[\s\S]+UNION ALL[\s\S]+FROM system_role_assignments`).
		WithArgs(userID, accountID, userID).
		WillReturnRows(sqlmock.NewRows([]string{
			"role_id", "account_id", "permission_id", "permission_key",
			"module_id", "scope", "source", "catalog_epoch",
			"catalog_checksum",
		}).AddRow(
			"018f1f64-7b2a-7c10-8bf2-3c40d5f67582",
			accountID,
			"018f1f64-7b2a-7c10-8bf2-3c40d5f67583",
			"content.articles.read.any",
			"content",
			"any",
			"account_role",
			7,
			make([]byte, 32),
		))

	grants, err := repository.EffectiveGrants(
		context.Background(),
		userID,
		accountID,
	)

	require.NoError(t, err)
	require.Len(t, grants, 1)
	require.Equal(t, ports.GrantSourceAccountRole, grants[0].Source)
	require.Equal(t, int64(7), grants[0].CatalogEpoch)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestPermissionRepositoryRejectsInjectedScopeOwnerMismatch(t *testing.T) {
	accountID := databaseid.ID(
		"018f1f64-7b2a-7c10-8bf2-3c40d5f67581",
	)
	_, err := mapEffectiveGrant(effectiveGrantRecord{
		RoleID: databaseid.ID(
			"018f1f64-7b2a-7c10-8bf2-3c40d5f67582",
		),
		AccountID: &accountID,
		PermissionID: databaseid.ID(
			"018f1f64-7b2a-7c10-8bf2-3c40d5f67583",
		),
		PermissionKey:   "system.users.manage.system",
		ModuleID:        "system",
		Scope:           string(authorizationdomain.PermissionScopeSystem),
		Source:          string(ports.GrantSourceAccountRole),
		CatalogEpoch:    1,
		CatalogChecksum: make([]byte, 32),
	}, accountID.String())

	require.ErrorIs(t, err, ErrInvalidEffectiveGrant)
}

func permissionRepositoryDatabase(
	t *testing.T,
) (*gormdb.DB, sqlmock.Sqlmock) {
	t.Helper()
	sqlDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })
	db, err := gormdb.Open(
		postgres.New(postgres.Config{Conn: sqlDB}),
		&gormdb.Config{DisableAutomaticPing: true},
	)
	require.NoError(t, err)
	return db, mock
}
