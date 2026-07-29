package gorm

import (
	"context"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
)

func TestPasswordRepositoryReadsHashOnly(t *testing.T) {
	t.Parallel()

	db, mock := sessionRepositoryDatabase(t)
	repository, err := NewPasswordRepository(db)
	require.NoError(t, err)
	userID := "018f1f64-7b2a-7c10-8bf2-3c40d5f67585"
	mock.ExpectQuery(regexp.QuoteMeta(
		`SELECT "password_hash" FROM "password_credentials" WHERE user_id = $1 ORDER BY "password_credentials"."user_id" LIMIT $2`,
	)).
		WithArgs(userID, 1).
		WillReturnRows(sqlmock.NewRows([]string{"password_hash"}).AddRow("argon2id-hash"))

	hash, err := repository.FindPasswordHash(context.Background(), userID)

	require.NoError(t, err)
	require.Equal(t, "argon2id-hash", hash)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestPasswordRepositoryChangeUpdatesChangedAt(t *testing.T) {
	t.Parallel()

	db, mock := sessionRepositoryDatabase(t)
	repository, err := NewPasswordRepository(db)
	require.NoError(t, err)
	userID := "018f1f64-7b2a-7c10-8bf2-3c40d5f67585"
	changedAt := time.Date(2026, time.July, 30, 6, 0, 0, 0, time.UTC)
	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta(
		`UPDATE "password_credentials" SET "changed_at"=$1,"password_hash"=$2 WHERE user_id = $3`,
	)).
		WithArgs(changedAt, "new-hash", userID).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	require.NoError(
		t,
		repository.ChangePassword(context.Background(), userID, "new-hash", changedAt),
	)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestPasswordRepositoryRehashPreservesChangedAt(t *testing.T) {
	t.Parallel()

	db, mock := sessionRepositoryDatabase(t)
	repository, err := NewPasswordRepository(db)
	require.NoError(t, err)
	userID := "018f1f64-7b2a-7c10-8bf2-3c40d5f67585"
	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta(
		`UPDATE "password_credentials" SET "password_hash"=$1 WHERE user_id = $2`,
	)).
		WithArgs("rehash", userID).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	require.NoError(
		t,
		repository.RehashPassword(context.Background(), userID, "rehash"),
	)
	require.NoError(t, mock.ExpectationsWereMet())
}
