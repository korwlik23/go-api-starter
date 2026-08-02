package schedules

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	publishingdomain "github.com/korwlik23/go-api-starter/internal/modules/publishing/domain"
	"github.com/korwlik23/go-api-starter/internal/modules/publishing/ports"
	"github.com/stretchr/testify/require"
)

const (
	scheduleAccountID     = "018f1f64-7b2a-7c10-8bf2-3c40d5f67580"
	scheduleTranslationID = "018f1f64-7b2a-7c10-8bf2-3c40d5f67581"
)

func TestM3A8CreateIsIdempotentByAccountKey(t *testing.T) {
	t.Parallel()

	repository := &scheduleRepositoryFake{existing: publishingdomain.PublicationSchedule{ID: "existing", State: publishingdomain.ScheduleSucceeded}}
	service := newScheduleService(t, repository, &scheduleExecutorFake{}, &scheduleAuditorFake{})
	schedule, err := service.Create(context.Background(), CreateInput{
		AccountID: scheduleAccountID, ActorUserID: scheduleTranslationID, RequestID: "schedule-create",
		TranslationID: scheduleTranslationID, PublishAt: scheduleNow().Add(time.Hour), ExpectedVersion: 2,
		IdempotencyKey: "schedule-key",
	})
	require.NoError(t, err)
	require.Equal(t, "existing", schedule.ID)
	require.Equal(t, 0, repository.createCalls)
}

func TestM3A8RunDueRetriesThenFailsAfterBoundedAttempts(t *testing.T) {
	t.Parallel()

	schedule := publishingdomain.PublicationSchedule{
		ID: "018f1f64-7b2a-7c10-8bf2-3c40d5f67582", AccountID: scheduleAccountID,
		TranslationID: scheduleTranslationID, PublishAt: scheduleNow().Add(-time.Minute),
		ExpectedVersion: 2, IdempotencyKey: "schedule-key", State: publishingdomain.SchedulePending,
		Attempts: 1,
	}
	repository := &scheduleRepositoryFake{due: []publishingdomain.PublicationSchedule{schedule}}
	service := newScheduleService(t, repository, &scheduleExecutorFake{err: errors.New("upstream unavailable")}, &scheduleAuditorFake{})
	result, err := service.RunDue(context.Background(), RunDueInput{Limit: 10})
	require.NoError(t, err)
	require.Equal(t, 1, result.Failed)
	require.Equal(t, publishingdomain.ScheduleFailed, repository.finished.State)
	require.NotEmpty(t, repository.finished.FailureReason)
}

func TestM3A8RunDueSucceedsExactlyOnceAfterClaim(t *testing.T) {
	t.Parallel()

	schedule := publishingdomain.PublicationSchedule{
		ID: "018f1f64-7b2a-7c10-8bf2-3c40d5f67582", AccountID: scheduleAccountID,
		TranslationID: scheduleTranslationID, PublishAt: scheduleNow().Add(-time.Minute),
		ExpectedVersion: 2, IdempotencyKey: "schedule-key", State: publishingdomain.SchedulePending,
	}
	repository := &scheduleRepositoryFake{due: []publishingdomain.PublicationSchedule{schedule}}
	auditor := &scheduleAuditorFake{}
	executor := &scheduleExecutorFake{}
	service := newScheduleService(t, repository, executor, auditor)
	result, err := service.RunDue(context.Background(), RunDueInput{Limit: 10})
	require.NoError(t, err)
	require.Equal(t, 1, result.Succeeded)
	require.Equal(t, 1, executor.calls)
	require.Equal(t, publishingdomain.ScheduleSucceeded, repository.finished.State)
	require.Equal(t, "publishing.schedule.succeeded", auditor.event.Action)
}

func newScheduleService(t *testing.T, repository *scheduleRepositoryFake, executor *scheduleExecutorFake, auditor *scheduleAuditorFake) *Service {
	t.Helper()
	service, err := NewService(Dependencies{
		Repository: repository, Auditor: auditor, Executor: executor,
		NewID: scheduleID, Now: scheduleNow, LeaseDuration: 5 * time.Minute, MaxAttempts: 2,
	})
	require.NoError(t, err)
	return service
}

func scheduleNow() time.Time { return time.Date(2026, 8, 2, 14, 0, 0, 0, time.UTC) }

var scheduleIDSequence int32

func scheduleID() (string, error) {
	sequence := atomic.AddInt32(&scheduleIDSequence, 1)
	return "018f1f64-7b2a-7c10-8bf2-3c40d5f6758" + string(rune('0'+sequence)), nil
}

type scheduleRepositoryFake struct {
	existing    publishingdomain.PublicationSchedule
	due         []publishingdomain.PublicationSchedule
	createCalls int
	finished    ports.ScheduleFinishCommand
}

func (repository *scheduleRepositoryFake) Create(context.Context, ports.ScheduleCreateCommand) error {
	repository.createCalls++
	return nil
}
func (repository *scheduleRepositoryFake) FindByIdempotency(context.Context, string, string) (publishingdomain.PublicationSchedule, error) {
	if repository.existing.ID == "" {
		return publishingdomain.PublicationSchedule{}, ports.ErrScheduleNotFound
	}
	return repository.existing, nil
}
func (repository *scheduleRepositoryFake) List(context.Context, ports.ScheduleListQuery) (ports.SchedulePage, error) {
	return ports.SchedulePage{}, nil
}
func (repository *scheduleRepositoryFake) Cancel(context.Context, ports.ScheduleCancelCommand) error {
	return nil
}
func (repository *scheduleRepositoryFake) ClaimDue(context.Context, time.Time, time.Time, int) ([]publishingdomain.PublicationSchedule, error) {
	for index := range repository.due {
		repository.due[index].Attempts++
	}
	return repository.due, nil
}
func (repository *scheduleRepositoryFake) Finish(_ context.Context, command ports.ScheduleFinishCommand) error {
	repository.finished = command
	return nil
}

type scheduleExecutorFake struct {
	calls int
	err   error
}

func (executor *scheduleExecutorFake) Execute(context.Context, publishingdomain.PublicationSchedule) error {
	executor.calls++
	return executor.err
}

type scheduleAuditorFake struct {
	event ScheduleAuditEvent
	err   error
}

func (auditor *scheduleAuditorFake) RecordScheduleChange(_ context.Context, event ScheduleAuditEvent) error {
	auditor.event = event
	return auditor.err
}
