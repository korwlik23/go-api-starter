package seed

import (
	"context"
	"encoding/json"
	"time"
)

func (s *store) seedOperations(ctx context.Context, seed Config, userIDs map[string]string) error {
	now := seedPublishedAt
	for _, fixture := range []struct {
		key, title, body, kind string
		read                   bool
	}{
		{key: "welcome", title: "Welcome", body: "The local seed is ready.", kind: "info", read: true},
		{key: "review", title: "Review required", body: "A demo article is waiting for review.", kind: "publishing.review", read: false},
	} {
		var readAt any
		if fixture.read {
			readAt = now
		}
		if err := s.ensure(ctx, "notifications", map[string]any{"id": s.ids.Record("notification", fixture.key)}, map[string]any{
			"id": s.ids.Record("notification", fixture.key), "user_id": userIDs["owner"], "title": fixture.title, "body": fixture.body, "kind": fixture.kind, "read_at": readAt,
		}); err != nil {
			return err
		}
	}
	jobs := []struct {
		key, state string
		attempts   int
		errText    any
	}{
		{key: "seed.cache-warm", state: "done", attempts: 1, errText: nil},
		{key: "seed.content-audit", state: "queued", attempts: 0, errText: nil},
		{key: "seed.preview-refresh", state: "failed", attempts: 2, errText: "demo failure for retry UI"},
	}
	for _, job := range jobs {
		payload, err := json.Marshal(map[string]any{"source": "local-seed", "fixture": job.key})
		if err != nil {
			return err
		}
		if err := s.ensure(ctx, "operation_jobs", map[string]any{"id": s.ids.Record("job", job.key)}, map[string]any{
			"id": s.ids.Record("job", job.key), "name": job.key, "payload": json.RawMessage(payload), "state": job.state, "attempts": job.attempts, "last_error": job.errText,
		}); err != nil {
			return err
		}
	}
	for _, setting := range []struct{ key, value string }{
		{key: "site.name", value: "Go Starter Local"},
		{key: "site.default_robots", value: "index,follow"},
	} {
		if err := s.ensure(ctx, "system_settings", map[string]any{"key": setting.key}, map[string]any{"key": setting.key, "value": setting.value, "version": int64(1)}); err != nil {
			return err
		}
	}
	return s.seedAudit(ctx, seed, userIDs["owner"])
}

func (s *store) seedAudit(ctx context.Context, seed Config, ownerID string) error {
	data, err := json.Marshal(map[string]any{"source": "local-seed", "profile": "full-local", "secrets": false})
	if err != nil {
		return err
	}
	return s.ensure(ctx, "audit_events", map[string]any{"id": s.ids.Record("audit", "seed")}, map[string]any{
		"id": s.ids.Record("audit", "seed"), "scope": "account", "account_id": seed.AccountID, "actor_type": "user", "actor_id": ownerID,
		"action": "seed.local.completed", "operation_id": s.ids.Record("operation", "seed"), "request_id": "seed-local", "event_data": string(data),
		"occurred_at": seedPublishedAt, "recorded_at": time.Now().UTC(),
	})
}
