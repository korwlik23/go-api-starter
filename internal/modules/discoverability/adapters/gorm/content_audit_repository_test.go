package gorm

import (
	"context"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	discoverabilitydomain "github.com/korwlik23/go-api-starter/internal/modules/discoverability/domain"
	"github.com/korwlik23/go-api-starter/internal/modules/discoverability/ports"
	"github.com/stretchr/testify/require"
)

func TestM3B6ContentAuditRepositoryListsAccountScopedRows(t *testing.T) {
	t.Parallel()
	db, mock := discoverabilityRepositoryDatabase(t)
	repository, err := NewContentAuditRepository(db)
	require.NoError(t, err)
	now := time.Date(2026, 8, 2, 12, 0, 0, 0, time.UTC)
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT id, account_id, content_id, translation_id, locale_id, content_version, translation_version, status, checks, created_at, updated_at FROM "discoverability_content_audits" WHERE account_id = $1 ORDER BY updated_at DESC, id DESC LIMIT $2`)).
		WithArgs("018f1f64-7b2a-7c10-8bf2-3c40d5f67580", 26).
		WillReturnRows(sqlmock.NewRows([]string{"id", "account_id", "content_id", "translation_id", "locale_id", "content_version", "translation_version", "status", "checks", "created_at", "updated_at"}).AddRow(
			"018f1f64-7b2a-7c10-8bf2-3c40d5f67581", "018f1f64-7b2a-7c10-8bf2-3c40d5f67580", "018f1f64-7b2a-7c10-8bf2-3c40d5f67582", "018f1f64-7b2a-7c10-8bf2-3c40d5f67583", "018f1f64-7b2a-7c10-8bf2-3c40d5f67584", 2, 3, string(discoverabilitydomain.AuditStatusWarning), []byte(`[{"rule_key":"geo.definition","status":"warning","severity":"warning","observed_value":"missing","message":"missing"}]`), now, now,
		))
	page, err := repository.List(context.Background(), ports.ContentAuditListQuery{AccountID: "018f1f64-7b2a-7c10-8bf2-3c40d5f67580", Limit: 25})
	require.NoError(t, err)
	require.Len(t, page.Items, 1)
	require.Equal(t, discoverabilitydomain.AuditStatusWarning, page.Items[0].Status)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestM3B6ContentAuditRepositoryMapsMissingRowsToNotFound(t *testing.T) {
	t.Parallel()
	db, mock := discoverabilityRepositoryDatabase(t)
	repository, err := NewContentAuditRepository(db)
	require.NoError(t, err)
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT id, account_id, content_id, translation_id, locale_id, content_version, translation_version, status, checks, created_at, updated_at FROM "discoverability_content_audits" WHERE account_id = $1 AND id = $2 LIMIT $3`)).
		WithArgs("018f1f64-7b2a-7c10-8bf2-3c40d5f67580", "018f1f64-7b2a-7c10-8bf2-3c40d5f67581", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id"}))
	_, err = repository.Get(context.Background(), "018f1f64-7b2a-7c10-8bf2-3c40d5f67580", "018f1f64-7b2a-7c10-8bf2-3c40d5f67581")
	require.ErrorIs(t, err, ports.ErrContentAuditNotFound)
}
