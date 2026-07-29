package gorm

import (
	"context"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/korwlik23/go-api-starter/internal/modules/authorization/ports"
	"github.com/stretchr/testify/require"
)

func TestAssignmentRepositoryUsesInsertDeleteOnlyWithTrustedAccountScope(
	t *testing.T,
) {
	db, mock := permissionRepositoryDatabase(t)
	repository, err := NewAssignmentRepository(db)
	require.NoError(t, err)
	accountID := "018f1f64-7b2a-7c10-8bf2-3c40d5f67581"
	membershipID := "018f1f64-7b2a-7c10-8bf2-3c40d5f67582"
	roleID := "018f1f64-7b2a-7c10-8bf2-3c40d5f67583"
	now := time.Date(2026, time.July, 30, 12, 0, 0, 0, time.UTC)

	mock.ExpectBegin()
	mock.ExpectExec(`INSERT INTO "membership_roles"`).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()
	require.NoError(t, repository.AssignMembershipRole(
		context.Background(),
		accountID,
		membershipID,
		roleID,
		now,
	))

	mock.ExpectExec(`DELETE FROM membership_roles`).
		WithArgs(accountID, membershipID, roleID).
		WillReturnResult(sqlmock.NewResult(0, 1))
	require.NoError(t, repository.RevokeMembershipRole(
		context.Background(),
		accountID,
		membershipID,
		roleID,
	))
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestAssignmentRepositoryFailsClosedWhenExactAssignmentIsMissing(
	t *testing.T,
) {
	db, mock := permissionRepositoryDatabase(t)
	repository, err := NewAssignmentRepository(db)
	require.NoError(t, err)
	mock.ExpectExec(`DELETE FROM system_role_assignments`).
		WillReturnResult(sqlmock.NewResult(0, 0))

	err = repository.RevokeSystemRole(
		context.Background(),
		"018f1f64-7b2a-7c10-8bf2-3c40d5f67581",
		"018f1f64-7b2a-7c10-8bf2-3c40d5f67582",
	)

	require.ErrorIs(t, err, ports.ErrAssignmentConflict)
}
