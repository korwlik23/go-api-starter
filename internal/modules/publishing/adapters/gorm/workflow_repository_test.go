package gorm

import (
	"context"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/korwlik23/go-api-starter/internal/modules/publishing/ports"
	"github.com/stretchr/testify/require"
)

func TestM3A7WorkflowRepositoryAppendsScopedEvent(t *testing.T) {
	t.Parallel()

	db, mock := contentRepositoryDatabase(t)
	repository, err := NewWorkflowRepository(db)
	require.NoError(t, err)
	mock.ExpectBegin()
	mock.ExpectExec(`INSERT INTO "workflow_events"`).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	err = repository.AppendEvent(context.Background(), ports.WorkflowEvent{
		ID:            "018f1f64-7b2a-7c10-8bf2-3c40d5f67580",
		AccountID:     "018f1f64-7b2a-7c10-8bf2-3c40d5f67581",
		ContentID:     "018f1f64-7b2a-7c10-8bf2-3c40d5f67582",
		TranslationID: "018f1f64-7b2a-7c10-8bf2-3c40d5f67583",
		ActorID:       "018f1f64-7b2a-7c10-8bf2-3c40d5f67584",
		FromStatus:    "draft", ToStatus: "review", RequestID: "workflow-request",
		OccurredAt: time.Date(2026, 8, 2, 13, 0, 0, 0, time.UTC),
	})
	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}
