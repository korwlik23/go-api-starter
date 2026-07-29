package domain

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestNewEventAcceptsBoundedSanitizedPayload(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 7, 30, 10, 0, 0, 0, time.UTC)
	event, err := NewEvent(EventInput{
		ID:          "018f1f64-7b2a-7c10-8bf2-3c40d5f67585",
		Scope:       ScopeAccount,
		AccountID:   "018f1f64-7b2a-7c10-8bf2-3c40d5f67586",
		Actor:       Actor{Type: ActorUser, ID: "018f1f64-7b2a-7c10-8bf2-3c40d5f67587"},
		Action:      "authorization.role.assigned",
		OperationID: "018f1f64-7b2a-7c10-8bf2-3c40d5f67588",
		RequestID:   "request-123",
		Data: map[string]any{
			"role_id": "018f1f64-7b2a-7c10-8bf2-3c40d5f67589",
			"outcome": "success",
			"counts":  map[string]any{"affected_principals": 2},
		},
		OccurredAt: now,
	})

	require.NoError(t, err)
	require.Equal(t, now, event.OccurredAt)
	require.JSONEq(
		t,
		`{"counts":{"affected_principals":2},"outcome":"success","role_id":"018f1f64-7b2a-7c10-8bf2-3c40d5f67589"}`,
		string(event.Data),
	)
}

func TestNewEventRejectsInvalidScopeActorAndSensitivePayload(t *testing.T) {
	t.Parallel()

	base := EventInput{
		ID:          "event-id",
		Scope:       ScopeSystem,
		Actor:       Actor{Type: ActorSystem},
		Action:      "identity.login.completed",
		OperationID: "operation-id",
		Data:        map[string]any{"outcome": "success"},
		OccurredAt:  time.Now(),
	}
	tests := map[string]func(*EventInput){
		"system scope with account": func(input *EventInput) {
			input.AccountID = "account-id"
		},
		"account scope without account": func(input *EventInput) {
			input.Scope = ScopeAccount
		},
		"user actor without ID": func(input *EventInput) {
			input.Actor = Actor{Type: ActorUser}
		},
		"system actor with ID": func(input *EventInput) {
			input.Actor = Actor{Type: ActorSystem, ID: "user-id"}
		},
		"password field": func(input *EventInput) {
			input.Data = map[string]any{"password_hash": "private"}
		},
		"nested token field": func(input *EventInput) {
			input.Data = map[string]any{
				"delivery": map[string]any{"reset_token": "private"},
			}
		},
		"PII field": func(input *EventInput) {
			input.Data = map[string]any{"email": "private@example.test"}
		},
		"non canonical action": func(input *EventInput) {
			input.Action = "Identity.Login"
		},
	}
	for name, mutate := range tests {
		name, mutate := name, mutate
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			input := base
			mutate(&input)
			_, err := NewEvent(input)
			require.ErrorIs(t, err, ErrInvalidEvent)
		})
	}
}
