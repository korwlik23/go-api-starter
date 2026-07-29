package gorm

import (
	"context"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/korwlik23/go-api-starter/internal/modules/authorization/ports"
	"github.com/stretchr/testify/require"
)

func TestPrincipalVersionRepositoryUsesScopedAtomicCASBumps(t *testing.T) {
	db, mock := permissionRepositoryDatabase(t)
	repository, err := NewPrincipalVersionRepository(db)
	require.NoError(t, err)
	accountID := "018f1f64-7b2a-7c10-8bf2-3c40d5f67581"
	membershipID := "018f1f64-7b2a-7c10-8bf2-3c40d5f67582"
	mock.ExpectBegin()
	mock.ExpectExec(`UPDATE "memberships"`).
		WithArgs(int64(4), accountID, membershipID, int64(3)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	version, err := repository.BumpMembershipVersion(
		context.Background(),
		accountID,
		membershipID,
		3,
	)

	require.NoError(t, err)
	require.Equal(t, int64(4), version)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestPrincipalVersionRepositoryFailsClosedOnCASConflict(t *testing.T) {
	db, mock := permissionRepositoryDatabase(t)
	repository, err := NewPrincipalVersionRepository(db)
	require.NoError(t, err)
	mock.ExpectBegin()
	mock.ExpectExec(`UPDATE "users"`).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectCommit()

	_, err = repository.BumpUserVersion(
		context.Background(),
		"018f1f64-7b2a-7c10-8bf2-3c40d5f67581",
		3,
	)

	require.ErrorIs(t, err, ports.ErrPrincipalVersionConflict)
}
