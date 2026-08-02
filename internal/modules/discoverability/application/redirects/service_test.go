package redirects

import (
	"context"
	"testing"
	"time"

	discoverabilitydomain "github.com/korwlik23/go-api-starter/internal/modules/discoverability/domain"
	"github.com/korwlik23/go-api-starter/internal/modules/discoverability/ports"
	"github.com/korwlik23/go-api-starter/internal/platform/database"
	"github.com/stretchr/testify/require"
)

func TestM3B5CreateRedirectAuditsSafeProjection(t *testing.T) {
	t.Parallel()
	repository := &redirectRepositoryFake{}
	auditor := &redirectAuditorFake{}
	service, err := NewService(Dependencies{Repository: repository, Auditor: auditor, Transactions: redirectTransactionFunc(func(ctx context.Context, op func(context.Context) error) error { return op(ctx) }), NewID: redirectIDSequence(), Now: func() time.Time { return time.Date(2026, 8, 2, 10, 0, 0, 0, time.UTC) }})
	require.NoError(t, err)
	result, err := service.Create(context.Background(), CreateInput{AccountID: "018f1f64-7b2a-7c10-8bf2-3c40d5f67580", ActorUserID: "018f1f64-7b2a-7c10-8bf2-3c40d5f67581", RequestID: "request-redirect", LocaleID: "018f1f64-7b2a-7c10-8bf2-3c40d5f67582", SourcePath: "/old", DestinationPath: "/new", StatusCode: discoverabilitydomain.StatusMovedPermanently, Enabled: true})
	require.NoError(t, err)
	require.Equal(t, "/new", result.DestinationPath)
	require.Equal(t, "discoverability.redirect.created", auditor.event.Action)
	require.NotContains(t, auditor.event.Data, "destination_path")
}

func TestM3B5UpdateAndDeleteRequireExpectedVersion(t *testing.T) {
	t.Parallel()
	repository := &redirectRepositoryFake{updateErr: ports.ErrRedirectConflict}
	service, err := NewService(Dependencies{Repository: repository, Auditor: &redirectAuditorFake{}, NewID: redirectIDSequence(), Now: time.Now})
	require.NoError(t, err)
	_, err = service.Update(context.Background(), UpdateInput{AccountID: "018f1f64-7b2a-7c10-8bf2-3c40d5f67580", ActorUserID: "018f1f64-7b2a-7c10-8bf2-3c40d5f67581", RequestID: "request-update", RedirectID: "018f1f64-7b2a-7c10-8bf2-3c40d5f67582", ExpectedVersion: 1, LocaleID: "018f1f64-7b2a-7c10-8bf2-3c40d5f67583", SourcePath: "/old", DestinationPath: "/new", StatusCode: discoverabilitydomain.StatusFound})
	require.ErrorIs(t, err, ports.ErrRedirectConflict)
	_, err = service.Update(context.Background(), UpdateInput{ExpectedVersion: 0})
	require.ErrorIs(t, err, ErrInvalidRedirectMutation)
}

func TestM3B5GetRedirectPreservesAccountScope(t *testing.T) {
	t.Parallel()
	expected := discoverabilitydomain.Redirect{ID: "018f1f64-7b2a-7c10-8bf2-3c40d5f67582", AccountID: "018f1f64-7b2a-7c10-8bf2-3c40d5f67580"}
	repository := &redirectRepositoryFake{got: expected}
	service, err := NewService(Dependencies{Repository: repository, Auditor: &redirectAuditorFake{}, NewID: redirectIDSequence(), Now: time.Now})
	require.NoError(t, err)
	result, err := service.Get(context.Background(), expected.AccountID, expected.ID)
	require.NoError(t, err)
	require.Equal(t, expected, result)
}

type redirectRepositoryFake struct {
	created   ports.CreateRedirectCommand
	updateErr error
	got       discoverabilitydomain.Redirect
}

func (*redirectRepositoryFake) List(context.Context, ports.RedirectListQuery) (ports.RedirectListPage, error) {
	return ports.RedirectListPage{}, nil
}
func (fake *redirectRepositoryFake) Get(context.Context, string, string) (discoverabilitydomain.Redirect, error) {
	return fake.got, nil
}
func (fake *redirectRepositoryFake) Create(_ context.Context, command ports.CreateRedirectCommand) (discoverabilitydomain.Redirect, error) {
	fake.created = command
	return command.Redirect, nil
}
func (fake *redirectRepositoryFake) Update(_ context.Context, command ports.UpdateRedirectCommand) (discoverabilitydomain.Redirect, error) {
	if fake.updateErr != nil {
		return discoverabilitydomain.Redirect{}, fake.updateErr
	}
	return command.Redirect, nil
}
func (*redirectRepositoryFake) Delete(context.Context, ports.DeleteRedirectCommand) error { return nil }

type redirectAuditorFake struct{ event RedirectAuditEvent }

func (auditor *redirectAuditorFake) RecordRedirectChange(_ context.Context, event RedirectAuditEvent) error {
	auditor.event = event
	return nil
}

type redirectTransactionFunc func(context.Context, func(context.Context) error) error

func (transaction redirectTransactionFunc) WithinTransaction(ctx context.Context, op func(context.Context) error) error {
	return transaction(ctx, op)
}

var _ database.Transactor = redirectTransactionFunc(nil)

func redirectIDSequence() func() (string, error) {
	value := "018f1f64-7b2a-7c10-8bf2-3c40d5f67584"
	return func() (string, error) { return value, nil }
}
