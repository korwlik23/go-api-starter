package seed

import (
	"context"
	"encoding/json"
	"fmt"
)

func (s *store) seedDiscoverability(
	ctx context.Context,
	seed Config,
	localeIDs map[string]string,
	contentIDs map[string]string,
	translationIDs map[string]string,
) error {
	for _, tag := range []string{"th", "en", "ja"} {
		if err := s.ensure(ctx, "discoverability_seo_defaults", map[string]any{"account_id": seed.AccountID, "locale_id": localeIDs[tag]}, map[string]any{
			"id": s.ids.Record("seo-default", tag), "account_id": seed.AccountID, "locale_id": localeIDs[tag], "title": "Go Starter", "description": "Personal Go + Gin + PostgreSQL CMS starter.",
			"canonical_base": "https://tewarach-dev.me", "robots": "index,follow", "version": int64(1),
		}); err != nil {
			return err
		}
	}
	if err := s.ensure(ctx, "discoverability_redirects", map[string]any{"account_id": seed.AccountID, "locale_id": localeIDs["th"], "source_path": "/company"}, map[string]any{
		"id": s.ids.Record("redirect", "company"), "account_id": seed.AccountID, "locale_id": localeIDs["th"], "source_path": "/company", "destination_path": "/about", "status_code": int16(301), "enabled": true, "version": int64(1),
	}); err != nil {
		return err
	}
	checks, err := json.Marshal([]map[string]any{{"key": "title", "status": "pass"}, {"key": "description", "status": "pass"}, {"key": "answer", "status": "warning"}})
	if err != nil {
		return err
	}
	for _, key := range []string{"home", "about"} {
		translationID := translationIDs[key+":en"]
		if translationID == "" {
			return fmt.Errorf("seed discoverability translation %q is missing", key)
		}
		if err := s.ensure(ctx, "discoverability_content_audits", map[string]any{
			"account_id": seed.AccountID, "translation_id": translationID, "content_version": int64(1), "translation_version": int64(1),
		}, map[string]any{
			"id": s.ids.Record("content-audit", key), "account_id": seed.AccountID, "content_id": contentIDs[key], "translation_id": translationID, "locale_id": localeIDs["en"],
			"content_version": int64(1), "translation_version": int64(1), "status": "warning", "checks": json.RawMessage(checks),
		}); err != nil {
			return err
		}
	}
	return nil
}
