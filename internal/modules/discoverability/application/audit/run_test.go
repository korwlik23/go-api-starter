package audit

import (
	"context"
	"testing"
	"time"

	discoverabilitydomain "github.com/korwlik23/go-api-starter/internal/modules/discoverability/domain"
	discoverabilityports "github.com/korwlik23/go-api-starter/internal/modules/discoverability/ports"
	publishingdomain "github.com/korwlik23/go-api-starter/internal/modules/publishing/domain"
	publishingports "github.com/korwlik23/go-api-starter/internal/modules/publishing/ports"
	"github.com/stretchr/testify/require"
)

func TestM3B6RunAuditStoresContentRevisionAndTranslationVersion(t *testing.T) {
	t.Parallel()
	contentID := "018f1f64-7b2a-7c10-8bf2-3c40d5f67582"
	localeID := "018f1f64-7b2a-7c10-8bf2-3c40d5f67583"
	repository := &contentRepositoryFake{detail: publishingports.ContentDetail{ContentID: contentID, TranslationID: "018f1f64-7b2a-7c10-8bf2-3c40d5f67585", AccountID: "018f1f64-7b2a-7c10-8bf2-3c40d5f67580", LocaleID: localeID, Kind: publishingdomain.KindPage, ContentVersion: 7, TranslationVersion: 9, Title: "Title", Blocks: []publishingdomain.Block{{Type: publishingdomain.BlockText, Data: map[string]any{"text": "Visible"}}}}}
	auditRepository := &auditRepositoryFake{}
	service, err := NewService(Dependencies{Content: repository, Audits: auditRepository, NewID: func() (string, error) { return "018f1f64-7b2a-7c10-8bf2-3c40d5f67586", nil }, Now: func() time.Time { return time.Date(2026, 8, 2, 12, 0, 0, 0, time.UTC) }})
	require.NoError(t, err)
	result, err := service.Run(context.Background(), RunInput{AccountID: repository.detail.AccountID, ContentID: contentID, LocaleID: localeID})
	require.NoError(t, err)
	require.Equal(t, int64(7), result.ContentVersion)
	require.Equal(t, int64(9), result.TranslationVersion)
	require.Equal(t, int64(7), auditRepository.command.Audit.ContentVersion)
	require.Equal(t, int64(9), auditRepository.command.Audit.TranslationVersion)
}

func TestM3B6RunAuditRejectsCrossAccountDetail(t *testing.T) {
	t.Parallel()
	service, err := NewService(Dependencies{Content: &contentRepositoryFake{detail: publishingports.ContentDetail{ContentID: "018f1f64-7b2a-7c10-8bf2-3c40d5f67582", AccountID: "018f1f64-7b2a-7c10-8bf2-3c40d5f67581", LocaleID: "018f1f64-7b2a-7c10-8bf2-3c40d5f67583", ContentVersion: 1, TranslationVersion: 1}}, Audits: &auditRepositoryFake{}, NewID: func() (string, error) { return "018f1f64-7b2a-7c10-8bf2-3c40d5f67586", nil }, Now: time.Now})
	require.NoError(t, err)
	_, err = service.Run(context.Background(), RunInput{AccountID: "018f1f64-7b2a-7c10-8bf2-3c40d5f67580", ContentID: "018f1f64-7b2a-7c10-8bf2-3c40d5f67582", LocaleID: "018f1f64-7b2a-7c10-8bf2-3c40d5f67583"})
	require.ErrorIs(t, err, ErrInvalidAuditSubject)
}

type contentRepositoryFake struct {
	detail publishingports.ContentDetail
	err    error
}

func (fake *contentRepositoryFake) Get(context.Context, string, string, string) (publishingports.ContentDetail, error) {
	return fake.detail, fake.err
}

type auditRepositoryFake struct {
	command discoverabilityports.SaveContentAuditCommand
}

func (fake *auditRepositoryFake) Save(_ context.Context, command discoverabilityports.SaveContentAuditCommand) (discoverabilitydomain.ContentAudit, error) {
	fake.command = command
	return command.Audit, nil
}
func (*auditRepositoryFake) Get(context.Context, string, string) (discoverabilitydomain.ContentAudit, error) {
	return discoverabilitydomain.ContentAudit{}, nil
}
func (*auditRepositoryFake) List(context.Context, discoverabilityports.ContentAuditListQuery) (discoverabilityports.ContentAuditListPage, error) {
	return discoverabilityports.ContentAuditListPage{}, nil
}
