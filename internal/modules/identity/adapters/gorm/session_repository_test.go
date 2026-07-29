package gorm

import (
	"context"
	"database/sql/driver"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	identitydomain "github.com/korwlik23/go-api-starter/internal/modules/identity/domain"
	"github.com/korwlik23/go-api-starter/internal/modules/identity/ports"
	"github.com/stretchr/testify/require"
	postgresgorm "gorm.io/driver/postgres"
	gormdb "gorm.io/gorm"
)

func TestSessionRepositoryCreatesHashOnlyRecord(t *testing.T) {
	db, mock := sessionRepositoryDatabase(t)
	repository, err := NewSessionRepository(db)
	require.NoError(t, err)

	now := time.Date(2026, time.July, 30, 2, 0, 0, 0, time.UTC)
	sessionHash := identitydomain.TokenHash{1, 2, 3}
	csrfHash := identitydomain.TokenHash{4, 5, 6}
	session := identitydomain.Session{
		ID:                "018f1f64-7b2a-7c10-8bf2-3c40d5f67585",
		TokenHash:         sessionHash,
		CSRFTokenHash:     csrfHash,
		State:             identitydomain.SessionStateAnonymous,
		IdleExpiresAt:     now.Add(time.Minute),
		AbsoluteExpiresAt: now.Add(time.Hour),
		CreatedAt:         now,
	}

	mock.ExpectBegin()
	mock.ExpectExec(`INSERT INTO "sessions"`).
		WithArgs(
			session.AbsoluteExpiresAt,
			nil,
			session.CreatedAt,
			csrfHash[:],
			session.ID,
			session.IdleExpiresAt,
			nil,
			string(session.State),
			sessionHash[:],
			nil,
		).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	require.NoError(t, repository.Create(context.Background(), session))
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestSessionRepositoryFindsAndRevokesByHash(t *testing.T) {
	db, mock := sessionRepositoryDatabase(t)
	repository, err := NewSessionRepository(db)
	require.NoError(t, err)

	now := time.Date(2026, time.July, 30, 2, 0, 0, 0, time.UTC)
	sessionHash := identitydomain.TokenHash{1, 2, 3}
	csrfHash := identitydomain.TokenHash{4, 5, 6}
	rows := sqlmock.NewRows([]string{
		"id", "token_hash", "csrf_token_hash", "state", "user_id",
		"authorization_version", "idle_expires_at", "absolute_expires_at",
		"revoked_at", "created_at",
	}).AddRow(
		"018f1f64-7b2a-7c10-8bf2-3c40d5f67585",
		sessionHash[:],
		csrfHash[:],
		"authenticated",
		"018f1f64-7b2a-7c10-8bf2-3c40d5f67586",
		int64(7),
		now.Add(time.Minute),
		now.Add(time.Hour),
		nil,
		now,
	)
	mock.ExpectQuery(regexp.QuoteMeta(
		`SELECT * FROM "sessions" WHERE token_hash = $1 ORDER BY "sessions"."id" LIMIT $2`,
	)).
		WithArgs(sessionHash[:], 1).
		WillReturnRows(rows)

	found, err := repository.FindByTokenHash(context.Background(), sessionHash)
	require.NoError(t, err)
	require.Equal(t, int64(7), *found.AuthorizationVersion)
	require.Equal(t, "018f1f64-7b2a-7c10-8bf2-3c40d5f67586", *found.UserID)
	require.Equal(t, identitydomain.SessionStateAuthenticated, found.State)

	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta(
		`UPDATE "sessions" SET "revoked_at"=$1 WHERE token_hash = $2 AND revoked_at IS NULL`,
	)).
		WithArgs(now, sessionHash[:]).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	revoked, err := repository.RevokeByTokenHash(context.Background(), sessionHash, now)
	require.NoError(t, err)
	require.True(t, revoked)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestSessionRepositoryReturnsStableNotFound(t *testing.T) {
	db, mock := sessionRepositoryDatabase(t)
	repository, err := NewSessionRepository(db)
	require.NoError(t, err)
	sessionHash := identitydomain.TokenHash{1}

	mock.ExpectQuery(`SELECT \* FROM "sessions"`).
		WithArgs(sessionHash[:], 1).
		WillReturnError(gormdb.ErrRecordNotFound)

	_, err = repository.FindByTokenHash(context.Background(), sessionHash)
	require.ErrorIs(t, err, ports.ErrSessionNotFound)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestSessionRepositoryPurgesExpiredRowsInBoundedBatch(t *testing.T) {
	db, mock := sessionRepositoryDatabase(t)
	repository, err := NewSessionRepository(db)
	require.NoError(t, err)
	now := time.Date(2026, time.July, 30, 2, 0, 0, 0, time.UTC)

	mock.ExpectQuery(regexp.QuoteMeta(
		`SELECT "id" FROM "sessions" WHERE absolute_expires_at <= $1 ORDER BY absolute_expires_at ASC LIMIT $2`,
	)).
		WithArgs(now, 2).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).
			AddRow("018f1f64-7b2a-7c10-8bf2-3c40d5f67585").
			AddRow("018f1f64-7b2a-7c10-8bf2-3c40d5f67586"))
	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta(
		`DELETE FROM "sessions" WHERE id IN ($1,$2) AND absolute_expires_at <= $3`,
	)).
		WithArgs(
			"018f1f64-7b2a-7c10-8bf2-3c40d5f67585",
			"018f1f64-7b2a-7c10-8bf2-3c40d5f67586",
			now,
		).
		WillReturnResult(sqlmock.NewResult(0, 2))
	mock.ExpectCommit()

	deleted, err := repository.PurgeExpired(context.Background(), now, 2)
	require.NoError(t, err)
	require.Equal(t, int64(2), deleted)
	require.NoError(t, mock.ExpectationsWereMet())

	_, err = repository.PurgeExpired(context.Background(), now, 0)
	require.ErrorIs(t, err, ports.ErrInvalidPurgeLimit)
}

func TestSessionRepositoryEncodesDatabaseUUIDsPerDialect(t *testing.T) {
	const id = "018f1f64-7b2a-7c10-8bf2-3c40d5f67585"

	postgresValue, err := encodeDatabaseID("postgres", id)
	require.NoError(t, err)
	require.Equal(t, driver.Value(id), postgresValue)

	mysqlValue, err := encodeDatabaseID("mysql", id)
	require.NoError(t, err)
	require.Equal(t, []byte{
		0x01, 0x8f, 0x1f, 0x64, 0x7b, 0x2a, 0x7c, 0x10,
		0x8b, 0xf2, 0x3c, 0x40, 0xd5, 0xf6, 0x75, 0x85,
	}, mysqlValue)

	var scanned databaseID
	require.NoError(t, scanned.Scan(mysqlValue))
	require.Equal(t, id, scanned.String())

	_, err = encodeDatabaseID("mysql", "not-a-uuid")
	require.ErrorIs(t, err, ErrInvalidDatabaseID)
}

func sessionRepositoryDatabase(t *testing.T) (*gormdb.DB, sqlmock.Sqlmock) {
	t.Helper()

	sqlDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() {
		mock.ExpectClose()
		require.NoError(t, sqlDB.Close())
		require.NoError(t, mock.ExpectationsWereMet())
	})

	db, err := gormdb.Open(
		postgresgorm.New(postgresgorm.Config{Conn: sqlDB}),
		&gormdb.Config{DisableAutomaticPing: true},
	)
	require.NoError(t, err)
	return db, mock
}
