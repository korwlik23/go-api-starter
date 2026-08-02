package gorm

import (
	"context"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/korwlik23/go-api-starter/internal/modules/publishing/ports"
	"github.com/stretchr/testify/require"
)

func TestM3A8ScheduleRepositoryFindsIdempotentAccountKey(t *testing.T) {
	t.Parallel()

	db, mock := contentRepositoryDatabase(t)
	repository, err := NewScheduleRepository(db)
	require.NoError(t, err)
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT id, account_id, translation_id, publish_at, unpublish_at, expected_version, idempotency_key, state, lease_until, attempts, failure_reason FROM "publication_schedules" WHERE account_id = $1 AND idempotency_key = $2 LIMIT $3`)).
		WithArgs("018f1f64-7b2a-7c10-8bf2-3c40d5f67580", "schedule-key", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "account_id", "translation_id", "publish_at", "unpublish_at", "expected_version", "idempotency_key", "state", "lease_until", "attempts", "failure_reason"}).AddRow(
			"018f1f64-7b2a-7c10-8bf2-3c40d5f67581", "018f1f64-7b2a-7c10-8bf2-3c40d5f67580", "018f1f64-7b2a-7c10-8bf2-3c40d5f67582",
			time.Date(2026, 8, 2, 15, 0, 0, 0, time.UTC), nil, 2, "schedule-key", "pending", nil, 0, "",
		))
	schedule, err := repository.FindByIdempotency(context.Background(), "018f1f64-7b2a-7c10-8bf2-3c40d5f67580", "schedule-key")
	require.NoError(t, err)
	require.Equal(t, "schedule-key", schedule.IdempotencyKey)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestM3A8ScheduleRepositoryMapsFinishStaleLeaseToConflict(t *testing.T) {
	t.Parallel()

	db, mock := contentRepositoryDatabase(t)
	repository, err := NewScheduleRepository(db)
	require.NoError(t, err)
	mock.ExpectBegin()
	mock.ExpectExec(`UPDATE "publication_schedules" SET`).WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectCommit()
	err = repository.Finish(context.Background(), ports.ScheduleFinishCommand{
		AccountID: "018f1f64-7b2a-7c10-8bf2-3c40d5f67580", ScheduleID: "018f1f64-7b2a-7c10-8bf2-3c40d5f67581",
		ExpectedAttempts: 1, State: "succeeded", FinishedAt: time.Date(2026, 8, 2, 15, 0, 0, 0, time.UTC),
	})
	require.ErrorIs(t, err, ports.ErrScheduleConflict)
	require.NoError(t, mock.ExpectationsWereMet())
}
