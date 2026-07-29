package migrations_test

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/korwlik23/go-api-starter/internal/platform/config"
	"github.com/stretchr/testify/require"
)

func m17AssertAuditEventConstraints(
	t *testing.T,
	ctx context.Context,
	db *sql.DB,
	cfg config.DatabaseConfig,
	seedOffset byte,
) {
	t.Helper()

	m17AssertAuditEventRequiredValues(t, ctx, db, cfg, seedOffset)
	m17AssertAuditEventScopeAndActorConstraints(t, ctx, db, cfg, seedOffset)
	m17AssertAuditEventActionAndRequestConstraints(t, ctx, db, cfg, seedOffset)
	m17AssertAuditEventPayloadConstraints(t, ctx, db, cfg, seedOffset)
	m17AssertMySQLNonStrictAuditBounds(t, ctx, db, cfg, seedOffset)
}

func m17AssertAuditEventRequiredValues(t *testing.T, ctx context.Context, db *sql.DB, cfg config.DatabaseConfig, seedOffset byte) {
	t.Helper()

	base := m17NewAuditEvent(t, cfg, 40+seedOffset, "system", nil, "system", nil, "audit.required", nil, time.Now().UTC().Truncate(time.Microsecond))
	for index, field := range []string{"id", "scope", "actor_type", "action", "operation_id", "event_data"} {
		event := base
		event.id = m3UserID(t, cfg, byte(41+index)+seedOffset)
		switch field {
		case "id":
			event.id = nil
		case "scope":
			event.scope = ""
		case "actor_type":
			event.actorType = ""
		case "action":
			event.action = ""
		case "operation_id":
			event.operationID = nil
		case "event_data":
			event.eventData = ""
		}
		_, err := db.ExecContext(ctx, m17AuditEventInsertSQL(cfg), m17AuditEventArguments(event)...)
		require.Error(t, err, "%s must be explicit and valid", field)
	}

	_, err := db.ExecContext(ctx,
		"INSERT INTO audit_events (scope, actor_type, action, operation_id, event_data, occurred_at) VALUES ("+migrationPlaceholders(cfg, 6)+")",
		"system", "system", "audit.required", m3UserID(t, cfg, 55+seedOffset), `{}`, time.Now().UTC(),
	)
	require.Error(t, err, "event ID must not have a default")
	_, err = db.ExecContext(ctx,
		"INSERT INTO audit_events (id, scope, actor_type, action, operation_id, event_data) VALUES ("+migrationPlaceholders(cfg, 6)+")",
		m3UserID(t, cfg, 56+seedOffset), "system", "system", "audit.required", m3UserID(t, cfg, 57+seedOffset), `{}`,
	)
	require.Error(t, err, "occurred_at must not have a default")
}

func m17AssertAuditEventScopeAndActorConstraints(t *testing.T, ctx context.Context, db *sql.DB, cfg config.DatabaseConfig, seedOffset byte) {
	t.Helper()

	accountID := m1AccountID(t, cfg, 60+seedOffset)
	userID := m3UserID(t, cfg, 60+seedOffset)
	m2InsertAccount(t, ctx, db, cfg, accountID, fmt.Sprintf("m17-constraint-account-%d", seedOffset))
	m2InsertUser(t, ctx, db, cfg, userID, fmt.Sprintf("m17-constraint-%d@example.test", seedOffset))
	base := m17NewAuditEvent(t, cfg, 61+seedOffset, "account", accountID, "user", userID, "audit.constraint", nil, time.Now().UTC())

	invalid := []struct {
		name string
		edit func(*m17AuditEvent)
	}{
		{"system account", func(event *m17AuditEvent) { event.scope, event.accountID = "system", accountID }},
		{"account nil", func(event *m17AuditEvent) { event.accountID = nil }},
		{"unknown account", func(event *m17AuditEvent) { event.accountID = m1AccountID(t, cfg, 62+seedOffset) }},
		{"unknown scope", func(event *m17AuditEvent) { event.scope = "tenant" }},
		{"user nil", func(event *m17AuditEvent) { event.actorID = nil }},
		{"system user", func(event *m17AuditEvent) { event.actorType = "system" }},
		{"anonymous user", func(event *m17AuditEvent) { event.actorType = "anonymous" }},
		{"unknown user", func(event *m17AuditEvent) { event.actorID = m3UserID(t, cfg, 63+seedOffset) }},
		{"unknown actor", func(event *m17AuditEvent) { event.actorType = "service" }},
	}
	for index, test := range invalid {
		event := base
		event.id = m3UserID(t, cfg, byte(64+index)+seedOffset)
		test.edit(&event)
		before := m17TableCount(t, ctx, db)
		_, err := db.ExecContext(ctx, m17AuditEventInsertSQL(cfg), m17AuditEventArguments(event)...)
		require.Error(t, err, "%s must fail", test.name)
		require.Equal(t, before, m17TableCount(t, ctx, db))
	}
}

func m17AssertAuditEventActionAndRequestConstraints(t *testing.T, ctx context.Context, db *sql.DB, cfg config.DatabaseConfig, seedOffset byte) {
	t.Helper()

	base := m17NewAuditEvent(t, cfg, 80+seedOffset, "system", nil, "system", nil, "audit.action", nil, time.Now().UTC())
	maxAction := "a." + strings.Repeat("b", 126)
	require.Len(t, maxAction, 128)
	base.id = m3UserID(t, cfg, 81+seedOffset)
	base.action = maxAction
	m17InsertAuditEvent(t, ctx, db, cfg, base)

	for index, action := range []string{"audit", "audit..append", "Audit.append", "audit.append-unsafe", "audit.append_unsafe", "audit.123", "audit.ไทย", strings.Repeat("a", 129)} {
		event := base
		event.id = m3UserID(t, cfg, byte(82+index)+seedOffset)
		event.action = action
		_, err := db.ExecContext(ctx, m17AuditEventInsertSQL(cfg), m17AuditEventArguments(event)...)
		require.Error(t, err, "invalid action %d must fail", index)
	}

	maxRequest := "r" + strings.Repeat("a", 127)
	event := base
	event.id = m3UserID(t, cfg, 95+seedOffset)
	event.requestID = maxRequest
	m17InsertAuditEvent(t, ctx, db, cfg, event)
	for index, requestID := range []any{"", ".request", "request id", "request/id", "requestไทย", "r" + strings.Repeat("a", 128)} {
		event := base
		event.id = m3UserID(t, cfg, byte(96+index)+seedOffset)
		event.requestID = requestID
		_, err := db.ExecContext(ctx, m17AuditEventInsertSQL(cfg), m17AuditEventArguments(event)...)
		require.Error(t, err, "invalid request ID %d must fail", index)
	}
}

func m17AssertAuditEventPayloadConstraints(t *testing.T, ctx context.Context, db *sql.DB, cfg config.DatabaseConfig, seedOffset byte) {
	t.Helper()

	base := m17NewAuditEvent(t, cfg, 110+seedOffset, "system", nil, "system", nil, "audit.payload", nil, time.Now().UTC())
	base.eventData = `{}`
	m17InsertAuditEvent(t, ctx, db, cfg, base)
	maximum := m17JSONPayload(t, 16384)
	event := base
	event.id = m3UserID(t, cfg, 111+seedOffset)
	event.eventData = maximum
	m17InsertAuditEvent(t, ctx, db, cfg, event)
	for index, data := range []string{"{", "[]", `"scalar"`, "null", m17JSONPayload(t, 16385)} {
		event := base
		event.id = m3UserID(t, cfg, byte(112+index)+seedOffset)
		event.eventData = data
		before := m17TableCount(t, ctx, db)
		_, err := db.ExecContext(ctx, m17AuditEventInsertSQL(cfg), m17AuditEventArguments(event)...)
		require.Error(t, err, "invalid event data %d must fail", index)
		require.Equal(t, before, m17TableCount(t, ctx, db))
	}
}

func m17AssertMySQLNonStrictAuditBounds(t *testing.T, ctx context.Context, db *sql.DB, cfg config.DatabaseConfig, seedOffset byte) {
	t.Helper()
	if cfg.Driver != "mysql" {
		return
	}

	connection, err := db.Conn(ctx)
	require.NoError(t, err)
	var previousSQLMode string
	require.NoError(t, connection.QueryRowContext(ctx, "SELECT @@SESSION.sql_mode").Scan(&previousSQLMode))
	require.NoError(t, m9SetSQLMode(ctx, connection, ""))
	defer func() {
		restoreContext, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		require.NoError(t, m9SetSQLMode(restoreContext, connection, previousSQLMode))
		require.NoError(t, connection.Close())
	}()

	base := m17NewAuditEvent(t, cfg, 130+seedOffset, "system", nil, "system", nil, "audit.nonstrict", nil, time.Now().UTC())
	for index, edit := range []func(*m17AuditEvent){
		func(event *m17AuditEvent) { event.action = "a." + strings.Repeat("b", 128) },
		func(event *m17AuditEvent) { event.requestID = "r" + strings.Repeat("a", 128) },
		func(event *m17AuditEvent) { event.eventData = m17JSONPayload(t, 16385) },
	} {
		event := base
		event.id = m3UserID(t, cfg, byte(131+index)+seedOffset)
		edit(&event)
		before := m17AuditTableCountOnQueryer(t, ctx, connection)
		_, err = connection.ExecContext(ctx, m17AuditEventInsertSQL(cfg), m17AuditEventArguments(event)...)
		require.Error(t, err, "non-strict invalid input %d must not truncate into a valid event", index)
		require.Equal(t, before, m17AuditTableCountOnQueryer(t, ctx, connection))
	}
}

func m17JSONPayload(t *testing.T, length int) string {
	t.Helper()
	prefix, suffix := `{"data":"`, `"}`
	payload := prefix + strings.Repeat("x", length-len(prefix)-len(suffix)) + suffix
	require.Len(t, payload, length)
	return payload
}

type m17RowCounter interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

func m17AuditTableCountOnQueryer(t *testing.T, ctx context.Context, queryer m17RowCounter) int {
	t.Helper()
	var count int
	require.NoError(t, queryer.QueryRowContext(ctx, "SELECT COUNT(*) FROM audit_events").Scan(&count))
	return count
}
