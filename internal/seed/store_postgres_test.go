package seed

import (
	"context"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
	postgresgorm "gorm.io/driver/postgres"
	gormdb "gorm.io/gorm"
)

func TestEnsurePostgresUsesAStandaloneUpdateQuery(t *testing.T) {
	db, mock := seedPostgresDatabase(t)
	store, err := newStore(db, NewIDs())
	require.NoError(t, err)

	userID := "018f1f64-7b2a-7c10-8bf2-3c40d5f67586"
	mock.ExpectQuery(regexp.QuoteMeta(
		`SELECT count(*) FROM "user_profiles" WHERE "user_id" = $1`,
	)).WithArgs(userID).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	mock.ExpectExec(regexp.QuoteMeta(
		`UPDATE "user_profiles" SET "display_name" = $1, "preferred_locale" = $2, "timezone" = $3, "user_id" = $4, "version" = $5 WHERE "user_id" = $6`,
	)).WithArgs("Local Owner", "th", "Asia/Bangkok", userID, int64(1), userID).
		WillReturnResult(sqlmock.NewResult(0, 1))

	err = store.ensure(context.Background(), "user_profiles", map[string]any{
		"user_id": userID,
	}, map[string]any{
		"user_id":          userID,
		"display_name":     "Local Owner",
		"timezone":         "Asia/Bangkok",
		"preferred_locale": "th",
		"version":          int64(1),
	})

	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}

func seedPostgresDatabase(t *testing.T) (*gormdb.DB, sqlmock.Sqlmock) {
	t.Helper()
	sqlDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })
	db, err := gormdb.Open(
		postgresgorm.New(postgresgorm.Config{Conn: sqlDB}),
		&gormdb.Config{DisableAutomaticPing: true},
	)
	require.NoError(t, err)
	return db, mock
}
