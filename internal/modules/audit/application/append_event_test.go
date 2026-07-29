package application

import (
	"context"
	"testing"
	"time"

	auditdomain "github.com/korwlik23/go-api-starter/internal/modules/audit/domain"
	"github.com/stretchr/testify/require"
)

func TestAppendEventOwnsIdentifiersAndOccurrenceTime(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 7, 30, 10, 30, 0, 0, time.UTC)
	repository := &eventAppenderFake{}
	ids := []string{"event-id", "operation-id"}
	appender, err := NewAppendEvent(AppendDependencies{
		Repository: repository,
		NewID: func() (string, error) {
			id := ids[0]
			ids = ids[1:]
			return id, nil
		},
		Now: func() time.Time { return now },
	})
	require.NoError(t, err)

	result, err := appender.Execute(context.Background(), AppendCommand{
		Scope:  auditdomain.ScopeSystem,
		Actor:  auditdomain.Actor{Type: auditdomain.ActorSystem},
		Action: "operations.modules.reconciled",
		Data:   map[string]any{"revision": 2},
	})

	require.NoError(t, err)
	require.Equal(t, "event-id", repository.event.ID)
	require.Equal(t, "operation-id", repository.event.OperationID)
	require.Equal(t, now, repository.event.OccurredAt)
	require.Equal(t, repository.event, result)
}

type eventAppenderFake struct {
	event auditdomain.Event
	err   error
}

func (repository *eventAppenderFake) Append(
	_ context.Context,
	event auditdomain.Event,
) error {
	repository.event = event
	return repository.err
}
