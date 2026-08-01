package http

import (
	"encoding/json"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	auditdomain "github.com/korwlik23/go-api-starter/internal/modules/audit/domain"
	auditports "github.com/korwlik23/go-api-starter/internal/modules/audit/ports"
	"github.com/stretchr/testify/require"
)

func TestMapAuditEventRedactsSensitiveNestedValues(t *testing.T) {
	now := time.Date(2026, 8, 2, 1, 2, 3, 0, time.UTC)
	event := auditdomain.Event{
		ID: "event-1", OperationID: "operation-1", Scope: auditdomain.ScopeSystem,
		Actor:  auditdomain.Actor{Type: auditdomain.ActorUser, ID: "user-1"},
		Action: "localization.catalog.updated", OccurredAt: now, RecordedAt: now,
		Data: json.RawMessage(`{"category":"common","email":"hidden@example.com","items":[{"token":"hidden","version":2}]}`),
	}

	projection, err := mapAuditEvent(event)

	require.NoError(t, err)
	require.Equal(t, "common", projection.Data["category"])
	_, exists := projection.Data["email"]
	require.False(t, exists)
	items := projection.Data["items"].([]any)
	item := items[0].(map[string]any)
	_, exists = item["token"]
	require.False(t, exists)
}

func TestMapAuditEventRejectsUnknownActorType(t *testing.T) {
	now := time.Date(2026, 8, 2, 1, 2, 3, 0, time.UTC)
	_, err := mapAuditEvent(auditdomain.Event{
		ID: "event-1", OperationID: "operation-1", Scope: auditdomain.ScopeSystem,
		Actor:  auditdomain.Actor{Type: auditdomain.ActorType("service"), ID: ""},
		Action: "system.reconciled", OccurredAt: now, RecordedAt: now,
		Data: json.RawMessage(`{"ok":true}`),
	})

	require.Error(t, err)
}

func TestParseAuditFilterUsesStrictIntegerAndBoundedValues(t *testing.T) {
	gin.SetMode(gin.TestMode)
	request := httptest.NewRequest("GET", "/audit?limit=10oops", nil)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = request

	_, err := parseAuditFilter(ctx)

	require.Error(t, err)
}

func TestAuditCursorRoundTrip(t *testing.T) {
	cursor := auditports.Cursor{ID: "event-1", RecordedAt: time.Date(2026, 8, 2, 1, 2, 3, 0, time.UTC)}
	encoded, err := encodeCursor(cursor)
	require.NoError(t, err)
	decoded, err := decodeCursor(encoded)
	require.NoError(t, err)
	require.Equal(t, cursor, decoded)
}
