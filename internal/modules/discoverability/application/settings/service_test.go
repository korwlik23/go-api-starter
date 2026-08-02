package settings

import (
	"context"
	"testing"
	"time"

	discoverabilitydomain "github.com/korwlik23/go-api-starter/internal/modules/discoverability/domain"
	"github.com/korwlik23/go-api-starter/internal/modules/discoverability/ports"
	"github.com/stretchr/testify/require"
)

func TestM3B5UpdateSEODefaultsUsesOptimisticVersionAndAudit(t *testing.T) {
	t.Parallel()
	repository := &seoRepositoryFake{}
	auditor := &seoAuditorFake{}
	service, err := NewService(Dependencies{Repository: repository, Auditor: auditor, NewID: func() (string, error) { return "018f1f64-7b2a-7c10-8bf2-3c40d5f67583", nil }, Now: time.Now})
	require.NoError(t, err)
	result, err := service.Update(context.Background(), UpdateInput{AccountID: "018f1f64-7b2a-7c10-8bf2-3c40d5f67580", ActorUserID: "018f1f64-7b2a-7c10-8bf2-3c40d5f67581", RequestID: "request-seo", LocaleID: "018f1f64-7b2a-7c10-8bf2-3c40d5f67582", ExpectedVersion: 2, Title: "Title", Description: "Description", CanonicalBase: "https://example.test", Robots: discoverabilitydomain.RobotsNoIndexFollow})
	require.NoError(t, err)
	require.Equal(t, int64(3), result.Version)
	require.Equal(t, "discoverability.seo.updated", auditor.event.Action)
	require.Equal(t, int64(2), repository.command.ExpectedVersion)
}

type seoRepositoryFake struct {
	command ports.UpsertSEODefaultsCommand
}

func (*seoRepositoryFake) GetSEODefaults(context.Context, string, string) (discoverabilitydomain.SEODefaults, error) {
	return discoverabilitydomain.SEODefaults{}, nil
}
func (fake *seoRepositoryFake) UpsertSEODefaults(_ context.Context, command ports.UpsertSEODefaultsCommand) (discoverabilitydomain.SEODefaults, error) {
	fake.command = command
	return command.Defaults, nil
}

type seoAuditorFake struct{ event SEOAuditEvent }

func (auditor *seoAuditorFake) RecordSEOChange(_ context.Context, event SEOAuditEvent) error {
	auditor.event = event
	return nil
}
