package migrations_test

import (
	"context"
	"database/sql"
	"fmt"
	"testing"
	"time"

	"github.com/korwlik23/go-api-starter/internal/platform/config"
	"github.com/stretchr/testify/require"
)

type m17AuditEvent struct {
	id          any
	scope       string
	accountID   any
	actorType   string
	actorID     any
	action      string
	operationID any
	requestID   any
	eventData   string
	occurredAt  time.Time
}

func m17AssertAuditEventBehavior(
	t *testing.T,
	ctx context.Context,
	db *sql.DB,
	cfg config.DatabaseConfig,
	seedOffset byte,
) {
	t.Helper()

	m17AssertValidAuditEventShapes(t, ctx, db, cfg, seedOffset)
	m17AssertAuditEventCorrelationsAndOrdering(t, ctx, db, cfg, seedOffset)
	m17AssertAuditEventIdentity(t, ctx, db, cfg, seedOffset)
}

func m17AssertValidAuditEventShapes(
	t *testing.T,
	ctx context.Context,
	db *sql.DB,
	cfg config.DatabaseConfig,
	seedOffset byte,
) {
	t.Helper()

	accountID := m1AccountID(t, cfg, 10+seedOffset)
	userID := m3UserID(t, cfg, 10+seedOffset)
	m2InsertAccount(t, ctx, db, cfg, accountID, fmt.Sprintf("m17-behavior-account-%d", seedOffset))
	m2InsertUser(t, ctx, db, cfg, userID, fmt.Sprintf("m17-behavior-%d@example.test", seedOffset))
	baseTime := time.Date(2026, time.January, 2, 3, 4, 5, 123456000, time.UTC)

	tests := []m17AuditEvent{
		m17NewAuditEvent(t, cfg, 11+seedOffset, "system", nil, "system", nil, "modules.reconcile", nil, baseTime),
		m17NewAuditEvent(t, cfg, 12+seedOffset, "account", accountID, "user", userID, "identity.login.success", "req-17", baseTime.Add(time.Second)),
		m17NewAuditEvent(t, cfg, 13+seedOffset, "account", accountID, "system", nil, "accounts.bootstrap", nil, baseTime.Add(2*time.Second)),
		m17NewAuditEvent(t, cfg, 14+seedOffset, "system", nil, "user", userID, "identity.session.revoked", "request:17", baseTime.Add(3*time.Second)),
		m17NewAuditEvent(t, cfg, 15+seedOffset, "system", nil, "anonymous", nil, "identity.login.failed", nil, baseTime.Add(4*time.Second)),
	}
	for _, event := range tests {
		m17InsertAuditEvent(t, ctx, db, cfg, event)
	}
	require.Equal(t, len(tests), m17TableCount(t, ctx, db))
}

func m17AssertAuditEventCorrelationsAndOrdering(
	t *testing.T,
	ctx context.Context,
	db *sql.DB,
	cfg config.DatabaseConfig,
	seedOffset byte,
) {
	t.Helper()

	accountID := m1AccountID(t, cfg, 20+seedOffset)
	userID := m3UserID(t, cfg, 20+seedOffset)
	operationID := m3UserID(t, cfg, 21+seedOffset)
	m2InsertAccount(t, ctx, db, cfg, accountID, fmt.Sprintf("m17-order-account-%d", seedOffset))
	m2InsertUser(t, ctx, db, cfg, userID, fmt.Sprintf("m17-order-%d@example.test", seedOffset))
	first := m17NewAuditEvent(t, cfg, 22+seedOffset, "account", accountID, "user", userID, "identity.login.success", "same-request", time.Date(2024, time.January, 1, 0, 0, 0, 0, time.UTC))
	first.operationID = operationID
	second := m17NewAuditEvent(t, cfg, 23+seedOffset, "account", accountID, "user", userID, "identity.login.success", "same-request", first.occurredAt.Add(-24*time.Hour))
	second.operationID = operationID
	before := time.Now().UTC().Add(-time.Second)
	m17InsertAuditEvent(t, ctx, db, cfg, first)
	m17InsertAuditEvent(t, ctx, db, cfg, second)
	after := time.Now().UTC().Add(time.Second)

	rows, err := db.QueryContext(ctx,
		"SELECT id, occurred_at, recorded_at FROM audit_events WHERE account_id = "+migrationPlaceholder(cfg, 1)+
			" AND operation_id = "+migrationPlaceholder(cfg, 2)+" ORDER BY recorded_at, id",
		accountID, operationID,
	)
	require.NoError(t, err)
	defer func() { require.NoError(t, rows.Close()) }()

	var ids []string
	var occurred []time.Time
	for rows.Next() {
		var id any
		var occurredAt, recordedAt time.Time
		require.NoError(t, rows.Scan(&id, &occurredAt, &recordedAt))
		ids = append(ids, m15SnapshotID(id))
		occurred = append(occurred, occurredAt)
		require.False(t, recordedAt.Before(before))
		require.False(t, recordedAt.After(after))
	}
	require.NoError(t, rows.Err())
	require.Len(t, ids, 2, "correlation values must remain non-unique")
	require.NotEqual(t, occurred[0], occurred[1], "event time is independent from recorded order")
}

func m17AssertAuditEventIdentity(
	t *testing.T,
	ctx context.Context,
	db *sql.DB,
	cfg config.DatabaseConfig,
	seedOffset byte,
) {
	t.Helper()

	event := m17NewAuditEvent(t, cfg, 30+seedOffset, "system", nil, "system", nil, "audit.append", nil, time.Now().UTC().Truncate(time.Microsecond))
	m17InsertAuditEvent(t, ctx, db, cfg, event)
	_, err := db.ExecContext(ctx, m17AuditEventInsertSQL(cfg), m17AuditEventArguments(event)...)
	require.Error(t, err, "event IDs must be unique")
	require.Equal(t, 1, m17AuditEventCount(t, ctx, db, cfg, event.id))
}

func m17NewAuditEvent(
	t *testing.T,
	cfg config.DatabaseConfig,
	seed byte,
	scope string,
	accountID any,
	actorType string,
	actorID any,
	action string,
	requestID any,
	occurredAt time.Time,
) m17AuditEvent {
	t.Helper()
	return m17AuditEvent{
		id:          m3UserID(t, cfg, seed),
		scope:       scope,
		accountID:   accountID,
		actorType:   actorType,
		actorID:     actorID,
		action:      action,
		operationID: m3UserID(t, cfg, seed+80),
		requestID:   requestID,
		eventData:   `{"kind":"test"}`,
		occurredAt:  occurredAt,
	}
}

func m17InsertAuditEvent(t *testing.T, ctx context.Context, db *sql.DB, cfg config.DatabaseConfig, event m17AuditEvent) {
	t.Helper()
	_, err := db.ExecContext(ctx, m17AuditEventInsertSQL(cfg), m17AuditEventArguments(event)...)
	require.NoError(t, err)
}

func m17AuditEventInsertSQL(cfg config.DatabaseConfig) string {
	return "INSERT INTO audit_events (id, scope, account_id, actor_type, actor_id, action, operation_id, request_id, event_data, occurred_at) VALUES (" + migrationPlaceholders(cfg, 10) + ")"
}

func m17AuditEventArguments(event m17AuditEvent) []any {
	return []any{event.id, event.scope, event.accountID, event.actorType, event.actorID, event.action, event.operationID, event.requestID, event.eventData, event.occurredAt}
}

func m17AuditEventCount(t *testing.T, ctx context.Context, db *sql.DB, cfg config.DatabaseConfig, id any) int {
	t.Helper()
	var count int
	require.NoError(t, db.QueryRowContext(ctx, "SELECT COUNT(*) FROM audit_events WHERE id = "+migrationPlaceholder(cfg, 1), id).Scan(&count))
	return count
}
