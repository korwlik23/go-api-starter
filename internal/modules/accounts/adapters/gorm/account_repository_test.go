package gorm

import (
	"context"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	gormdb "gorm.io/gorm"
)

func TestAccountRepositoryResolvesOneActiveMembershipForAuthenticatedUser(
	t *testing.T,
) {
	db, mock := accountRepositoryDatabase(t)
	repository, err := NewAccountRepository(db)
	require.NoError(t, err)
	userID := "018f1f64-7b2a-7c10-8bf2-3c40d5f67586"
	mock.ExpectQuery(regexp.QuoteMeta(
		`SELECT accounts.id AS account_id, accounts.slug AS account_slug, accounts.status AS account_status, memberships.id AS membership_id, memberships.user_id, memberships.status AS membership_status, memberships.authorization_version FROM "memberships" JOIN accounts ON accounts.id = memberships.account_id WHERE memberships.user_id = $1 AND memberships.status = $2 AND accounts.status = $3 ORDER BY memberships.created_at ASC LIMIT $4`,
	)).WithArgs(userID, "active", "active", 2).
		WillReturnRows(sqlmock.NewRows([]string{
			"account_id", "account_slug", "account_status", "membership_id",
			"user_id", "membership_status", "authorization_version",
		}).AddRow(
			"018f1f64-7b2a-7c10-8bf2-3c40d5f67581",
			"personal",
			"active",
			"018f1f64-7b2a-7c10-8bf2-3c40d5f67582",
			userID,
			"active",
			1,
		))

	resolved, err := repository.ResolveActiveForUser(
		context.Background(),
		userID,
	)

	require.NoError(t, err)
	require.Equal(t, "personal", resolved.Account.Slug)
	require.Equal(t, userID, resolved.Membership.UserID)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestAccountRepositoryFailsClosedWhenPersonalAccountIsAmbiguous(t *testing.T) {
	db, mock := accountRepositoryDatabase(t)
	repository, err := NewAccountRepository(db)
	require.NoError(t, err)
	userID := "018f1f64-7b2a-7c10-8bf2-3c40d5f67586"
	rows := sqlmock.NewRows([]string{
		"account_id", "account_slug", "account_status", "membership_id",
		"user_id", "membership_status", "authorization_version",
	})
	accountIDs := []string{
		"018f1f64-7b2a-7c10-8bf2-3c40d5f67581",
		"018f1f64-7b2a-7c10-8bf2-3c40d5f67582",
	}
	membershipIDs := []string{
		"018f1f64-7b2a-7c10-8bf2-3c40d5f67591",
		"018f1f64-7b2a-7c10-8bf2-3c40d5f67592",
	}
	for index := range accountIDs {
		rows.AddRow(
			accountIDs[index],
			"personal",
			"active",
			membershipIDs[index],
			userID,
			"active",
			1,
		)
	}
	mock.ExpectQuery(`SELECT .* FROM "memberships"`).WillReturnRows(rows)

	_, err = repository.ResolveActiveForUser(context.Background(), userID)

	require.ErrorIs(t, err, ErrAmbiguousAccount)
}

func accountRepositoryDatabase(t *testing.T) (*gormdb.DB, sqlmock.Sqlmock) {
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
