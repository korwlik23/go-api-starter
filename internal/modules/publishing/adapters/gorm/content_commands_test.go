package gorm

import (
	"context"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/korwlik23/go-api-starter/internal/modules/publishing/ports"
	"github.com/stretchr/testify/require"
)

func TestM3A5SoftDeleteMapsStaleTranslationVersionToConflict(t *testing.T) {
	t.Parallel()

	db, mock := contentRepositoryDatabase(t)
	repository, err := NewContentRepository(db)
	require.NoError(t, err)
	mock.ExpectBegin()
	mock.ExpectExec(`UPDATE "content_translations" SET`).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectCommit()

	err = repository.SoftDelete(context.Background(), ports.DeleteContentCommand{
		AccountID:       "018f1f64-7b2a-7c10-8bf2-3c40d5f67585",
		ContentID:       "018f1f64-7b2a-7c10-8bf2-3c40d5f67587",
		TranslationID:   "018f1f64-7b2a-7c10-8bf2-3c40d5f67588",
		LocaleID:        "018f1f64-7b2a-7c10-8bf2-3c40d5f67586",
		ExpectedVersion: 2,
		DeletedAt:       time.Date(2026, 8, 2, 10, 0, 0, 0, time.UTC),
	})
	require.ErrorIs(t, err, ports.ErrContentConflict)
	require.NoError(t, mock.ExpectationsWereMet())
}
