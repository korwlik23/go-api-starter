package gorm

import (
	"context"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	identitydomain "github.com/korwlik23/go-api-starter/internal/modules/identity/domain"
	"github.com/korwlik23/go-api-starter/internal/modules/identity/ports"
	"github.com/stretchr/testify/require"
	gormdb "gorm.io/gorm"
)

func TestUserRepositoryFindsByNormalizedEmailWithoutCredentialFields(t *testing.T) {
	t.Parallel()

	db, mock := sessionRepositoryDatabase(t)
	repository, err := NewUserRepository(db)
	require.NoError(t, err)
	rows := sqlmock.NewRows([]string{
		"id", "normalized_email", "status", "authorization_version",
	}).AddRow(
		"018f1f64-7b2a-7c10-8bf2-3c40d5f67585",
		"person@example.com",
		"active",
		int64(7),
	)
	mock.ExpectQuery(regexp.QuoteMeta(
		`SELECT "id","normalized_email","status","authorization_version" FROM "users" WHERE normalized_email = $1 ORDER BY "users"."id" LIMIT $2`,
	)).
		WithArgs("person@example.com", 1).
		WillReturnRows(rows)

	user, err := repository.FindByNormalizedEmail(
		context.Background(),
		" Person@Example.COM ",
	)

	require.NoError(t, err)
	require.Equal(t, "person@example.com", user.Email)
	require.Equal(t, identitydomain.UserStatusActive, user.Status)
	require.Equal(t, int64(7), user.AuthorizationVersion)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestUserRepositoryReadsAuthorizationVersionByUserID(t *testing.T) {
	t.Parallel()

	db, mock := sessionRepositoryDatabase(t)
	repository, err := NewUserRepository(db)
	require.NoError(t, err)
	userID := "018f1f64-7b2a-7c10-8bf2-3c40d5f67585"
	mock.ExpectQuery(regexp.QuoteMeta(
		`SELECT "authorization_version" FROM "users" WHERE id = $1 ORDER BY "users"."id" LIMIT $2`,
	)).
		WithArgs(userID, 1).
		WillReturnRows(sqlmock.NewRows([]string{"authorization_version"}).AddRow(int64(9)))

	version, err := repository.CurrentAuthorizationVersion(context.Background(), userID)

	require.NoError(t, err)
	require.Equal(t, int64(9), version)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestUserRepositoryFindsSafeUserProjectionByID(t *testing.T) {
	t.Parallel()

	db, mock := sessionRepositoryDatabase(t)
	repository, err := NewUserRepository(db)
	require.NoError(t, err)
	userID := "018f1f64-7b2a-7c10-8bf2-3c40d5f67585"
	mock.ExpectQuery(`SELECT .* FROM "users" WHERE id = .*`).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "normalized_email", "status", "authorization_version",
		}).AddRow(userID, "person@example.com", "active", int64(7)))

	user, err := repository.FindByID(context.Background(), userID)

	require.NoError(t, err)
	require.Equal(t, "person@example.com", user.Email)
	require.Equal(t, int64(7), user.AuthorizationVersion)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestUserRepositoryReturnsStableNotFound(t *testing.T) {
	t.Parallel()

	db, mock := sessionRepositoryDatabase(t)
	repository, err := NewUserRepository(db)
	require.NoError(t, err)
	mock.ExpectQuery(`SELECT .* FROM "users"`).
		WithArgs("missing@example.com", 1).
		WillReturnError(gormdb.ErrRecordNotFound)

	_, err = repository.FindByNormalizedEmail(
		context.Background(),
		"missing@example.com",
	)

	require.ErrorIs(t, err, ports.ErrUserNotFound)
	require.NoError(t, mock.ExpectationsWereMet())
}
