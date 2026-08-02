package workflow

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	authdomain "github.com/korwlik23/go-api-starter/internal/modules/authorization/domain"
	publishingdomain "github.com/korwlik23/go-api-starter/internal/modules/publishing/domain"
	"github.com/korwlik23/go-api-starter/internal/modules/publishing/ports"
	"github.com/stretchr/testify/require"
)

func TestM3A7SubmitReviewRequiresWritePermissionAndAppendsAudit(t *testing.T) {
	t.Parallel()

	repository := &workflowRepositoryFake{detail: workflowDetail(publishingdomain.StatusDraft, 1)}
	auditor := &workflowAuditorFake{}
	service, err := NewService(Dependencies{
		Repository: repository, Authorizer: &workflowAuthorizerFake{allowed: true},
		Auditor: auditor, NewID: workflowSequenceID, Now: workflowNow,
	})
	require.NoError(t, err)

	result, err := service.Transition(context.Background(), TransitionInput{
		AccountID: workflowAccountID, ActorUserID: workflowActorID, RequestID: "workflow-submit",
		ContentID: workflowContentID, TranslationID: workflowTranslationID, LocaleID: workflowLocaleID,
		ExpectedVersion: 1, Action: "submit_review",
	})
	require.NoError(t, err)
	require.Equal(t, publishingdomain.StatusReview, result.TranslationStatus)
	require.Equal(t, "publishing.workflow.submit_review", auditor.event.Action)
	require.Equal(t, publishingdomain.StatusReview, repository.event.ToStatus)
}

func TestM3A7PublishDeniedAndStaleVersionConflict(t *testing.T) {
	t.Parallel()

	service, err := NewService(Dependencies{
		Repository: &workflowRepositoryFake{detail: workflowDetail(publishingdomain.StatusReview, 3)},
		Authorizer: &workflowAuthorizerFake{allowed: false}, Auditor: &workflowAuditorFake{},
		NewID: workflowSequenceID, Now: workflowNow,
	})
	require.NoError(t, err)
	_, err = service.Transition(context.Background(), TransitionInput{
		AccountID: workflowAccountID, ActorUserID: workflowActorID, RequestID: "workflow-publish",
		ContentID: workflowContentID, TranslationID: workflowTranslationID, LocaleID: workflowLocaleID,
		ExpectedVersion: 3, Action: "publish",
	})
	require.ErrorIs(t, err, ErrWorkflowDenied)

	_, err = service.Transition(context.Background(), TransitionInput{
		AccountID: workflowAccountID, ActorUserID: workflowActorID, RequestID: "workflow-stale",
		ContentID: workflowContentID, TranslationID: workflowTranslationID, LocaleID: workflowLocaleID,
		ExpectedVersion: 2, Action: "publish",
	})
	require.ErrorIs(t, err, ports.ErrWorkflowConflict)
}

func TestM3A7AuditFailurePropagatesForTransactionRollback(t *testing.T) {
	t.Parallel()

	auditErr := errors.New("audit unavailable")
	repository := &workflowRepositoryFake{detail: workflowDetail(publishingdomain.StatusReview, 1)}
	service, err := NewService(Dependencies{
		Repository: repository, Authorizer: &workflowAuthorizerFake{allowed: true},
		Transactions: workflowTransactionFunc(func(ctx context.Context, operation func(context.Context) error) error { return operation(ctx) }),
		Auditor:      &workflowAuditorFake{err: auditErr}, NewID: workflowSequenceID, Now: workflowNow,
	})
	require.NoError(t, err)
	_, err = service.Transition(context.Background(), TransitionInput{
		AccountID: workflowAccountID, ActorUserID: workflowActorID, RequestID: "workflow-audit",
		ContentID: workflowContentID, TranslationID: workflowTranslationID, LocaleID: workflowLocaleID,
		ExpectedVersion: 1, Action: "approve",
	})
	require.ErrorIs(t, err, auditErr)
}

const (
	workflowAccountID     = "018f1f64-7b2a-7c10-8bf2-3c40d5f67580"
	workflowActorID       = "018f1f64-7b2a-7c10-8bf2-3c40d5f67581"
	workflowContentID     = "018f1f64-7b2a-7c10-8bf2-3c40d5f67582"
	workflowTranslationID = "018f1f64-7b2a-7c10-8bf2-3c40d5f67583"
	workflowLocaleID      = "018f1f64-7b2a-7c10-8bf2-3c40d5f67584"
)

func workflowDetail(status publishingdomain.Status, version int64) ports.ContentDetail {
	return ports.ContentDetail{
		ContentID: workflowContentID, TranslationID: workflowTranslationID,
		AccountID: workflowAccountID, LocaleID: workflowLocaleID,
		Kind: publishingdomain.KindPage, ContentKey: "home",
		ContentStatus: status, ContentVersion: version,
		TranslationStatus: status, TranslationVersion: version,
		Slug: "home", Path: "/home", Title: "Home", UpdatedAt: workflowNow(),
	}
}

func workflowNow() time.Time { return time.Date(2026, 8, 2, 13, 0, 0, 0, time.UTC) }

var workflowIDSequence int32

func workflowSequenceID() (string, error) {
	sequence := atomic.AddInt32(&workflowIDSequence, 1)
	return "018f1f64-7b2a-7c10-8bf2-3c40d5f6758" + string(rune('0'+sequence)), nil
}

type workflowAuthorizerFake struct{ allowed bool }

func (fake *workflowAuthorizerFake) Allowed(context.Context, string, string, authdomain.PermissionKey, string) (bool, error) {
	return fake.allowed, nil
}

type workflowAuditorFake struct {
	event WorkflowAuditEvent
	err   error
}

func (fake *workflowAuditorFake) RecordWorkflowChange(_ context.Context, event WorkflowAuditEvent) error {
	fake.event = event
	return fake.err
}

type workflowRepositoryFake struct {
	detail ports.ContentDetail
	event  ports.WorkflowEvent
}

func (fake *workflowRepositoryFake) Get(context.Context, string, string, string) (ports.ContentDetail, error) {
	return fake.detail, nil
}

func (fake *workflowRepositoryFake) Transition(_ context.Context, command ports.WorkflowTransitionCommand) (ports.ContentDetail, error) {
	fake.detail.TranslationStatus = command.ToStatus
	fake.detail.ContentStatus = command.ToStatus
	fake.detail.TranslationVersion = command.ExpectedVersion + 1
	fake.detail.ContentVersion++
	fake.detail.UpdatedAt = command.UpdatedAt
	return fake.detail, nil
}

func (fake *workflowRepositoryFake) AppendEvent(_ context.Context, event ports.WorkflowEvent) error {
	fake.event = event
	return nil
}

type workflowTransactionFunc func(context.Context, func(context.Context) error) error

func (transaction workflowTransactionFunc) WithinTransaction(ctx context.Context, operation func(context.Context) error) error {
	return transaction(ctx, operation)
}
