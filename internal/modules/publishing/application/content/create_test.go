package content

import (
	"context"
	"errors"
	"testing"
	"time"

	publishingdomain "github.com/korwlik23/go-api-starter/internal/modules/publishing/domain"
	"github.com/korwlik23/go-api-starter/internal/modules/publishing/ports"
	"github.com/korwlik23/go-api-starter/internal/platform/database"
	"github.com/stretchr/testify/require"
)

func TestM3A5CreateBuildsVersionedContentAndRedactedAudit(t *testing.T) {
	t.Parallel()

	repository := &mutationRepositoryFake{}
	auditor := &contentAuditorFake{}
	transaction := &mutationTransactionFake{}
	service, err := NewContentMutations(ContentMutationDependencies{
		Repository:   repository,
		Transactions: transaction,
		Auditor:      auditor,
		NewID:        sequenceID("018f1f64-7b2a-7c10-8bf2-3c40d5f6758"),
		Now: func() time.Time {
			return time.Date(2026, 8, 2, 10, 0, 0, 0, time.UTC)
		},
	})
	require.NoError(t, err)

	result, err := service.Create(context.Background(), CreateInput{
		AccountID:   "018f1f64-7b2a-7c10-8bf2-3c40d5f67580",
		ActorUserID: "018f1f64-7b2a-7c10-8bf2-3c40d5f67581",
		RequestID:   "request-1",
		Kind:        publishingdomain.KindPage,
		ContentKey:  "home",
		Locale:      "en",
		Slug:        "home",
		Path:        "/home",
		Title:       "Home",
		Blocks: []publishingdomain.Block{{
			Type: publishingdomain.BlockText,
			Data: map[string]any{"text": "Welcome"},
		}},
	})
	require.NoError(t, err)
	require.Equal(t, int64(1), repository.created.Item.Version)
	require.Equal(t, int64(1), repository.created.Translation.Version)
	require.Equal(t, "publishing.content.created", auditor.event.Action)
	require.NotContains(t, auditor.event.Data, "blocks")
	require.True(t, transaction.calls > 0)
	require.Equal(t, result.ContentID, repository.createdResult.ContentID)
}

func TestM3A5CreateRollsBackWhenAuditFails(t *testing.T) {
	t.Parallel()

	auditErr := errors.New("audit unavailable")
	repository := &mutationRepositoryFake{}
	service, err := NewContentMutations(ContentMutationDependencies{
		Repository: repository,
		Transactions: mutationTransactionFunc(func(
			ctx context.Context,
			op func(context.Context) error,
		) error {
			return op(ctx)
		}),
		Auditor: &contentAuditorFake{err: auditErr},
		NewID:   sequenceID("018f1f64-7b2a-7c10-8bf2-3c40d5f6758"),
		Now:     func() time.Time { return time.Now().UTC() },
	})
	require.NoError(t, err)

	_, err = service.Create(context.Background(), CreateInput{
		AccountID:   "018f1f64-7b2a-7c10-8bf2-3c40d5f67580",
		ActorUserID: "018f1f64-7b2a-7c10-8bf2-3c40d5f67581",
		RequestID:   "request-audit-failure",
		Kind:        publishingdomain.KindPost, ContentKey: "news", Locale: "en",
		Slug: "news", Path: "/news", Title: "News",
		Blocks: []publishingdomain.Block{{
			Type: publishingdomain.BlockText,
			Data: map[string]any{"text": "News"},
		}},
	})
	require.ErrorIs(t, err, auditErr)
}

func TestM3A5UpdateRequiresExpectedVersionAndPropagatesConflict(t *testing.T) {
	t.Parallel()

	repository := &mutationRepositoryFake{updateErr: ports.ErrContentConflict}
	service, err := NewContentMutations(ContentMutationDependencies{
		Repository: repository,
		Auditor:    &contentAuditorFake{},
		NewID:      sequenceID("018f1f64-7b2a-7c10-8bf2-3c40d5f6758"),
		Now:        func() time.Time { return time.Now().UTC() },
	})
	require.NoError(t, err)

	_, err = service.Update(context.Background(), UpdateInput{
		AccountID:     "018f1f64-7b2a-7c10-8bf2-3c40d5f67580",
		ActorUserID:   "018f1f64-7b2a-7c10-8bf2-3c40d5f67581",
		RequestID:     "request-2",
		ContentID:     "018f1f64-7b2a-7c10-8bf2-3c40d5f67582",
		TranslationID: "018f1f64-7b2a-7c10-8bf2-3c40d5f67583",
		Locale:        "en", Slug: "news", Path: "/news", Title: "News",
		ExpectedVersion: 2,
		Blocks:          []publishingdomain.Block{{Type: publishingdomain.BlockText, Data: map[string]any{"text": "News"}}},
	})
	require.ErrorIs(t, err, ports.ErrContentConflict)

	_, err = service.Update(context.Background(), UpdateInput{ExpectedVersion: 0})
	require.ErrorIs(t, err, ErrInvalidContentMutation)
}

type mutationRepositoryFake struct {
	created       ports.CreateContentCommand
	createdResult ports.ContentDetail
	updateErr     error
	deleteErr     error
}

func (repository *mutationRepositoryFake) Create(
	_ context.Context,
	command ports.CreateContentCommand,
) (ports.ContentDetail, error) {
	repository.created = command
	repository.createdResult = ports.ContentDetail{
		ContentID:          command.Item.ID,
		TranslationID:      command.Translation.ID,
		AccountID:          command.Item.AccountID,
		LocaleID:           command.Translation.Locale,
		ContentStatus:      command.Item.Status,
		TranslationStatus:  command.Translation.Status,
		ContentVersion:     command.Item.Version,
		TranslationVersion: command.Translation.Version,
	}
	return repository.createdResult, nil
}

func (repository *mutationRepositoryFake) Update(
	_ context.Context,
	_ ports.UpdateContentCommand,
) (ports.ContentDetail, error) {
	if repository.updateErr != nil {
		return ports.ContentDetail{}, repository.updateErr
	}
	return ports.ContentDetail{}, nil
}

func (repository *mutationRepositoryFake) SoftDelete(
	_ context.Context,
	_ ports.DeleteContentCommand,
) error {
	return repository.deleteErr
}

type contentAuditorFake struct {
	event ContentAuditEvent
	err   error
}

func (auditor *contentAuditorFake) RecordContentChange(
	_ context.Context,
	event ContentAuditEvent,
) error {
	auditor.event = event
	return auditor.err
}

type mutationTransactionFake struct{ calls int }

func (transaction *mutationTransactionFake) WithinTransaction(
	ctx context.Context,
	op func(context.Context) error,
) error {
	transaction.calls++
	return op(ctx)
}

type mutationTransactionFunc func(context.Context, func(context.Context) error) error

func (transaction mutationTransactionFunc) WithinTransaction(
	ctx context.Context,
	op func(context.Context) error,
) error {
	return transaction(ctx, op)
}

var _ database.Transactor = (*mutationTransactionFake)(nil)

func sequenceID(prefix string) func() (string, error) {
	sequence := 0
	return func() (string, error) {
		sequence++
		return prefix + string(rune('0'+sequence)), nil
	}
}
