package seed

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"
)

type contentFixture struct {
	Key    string
	Kind   string
	Status string
}

var demoContent = []contentFixture{
	{Key: "home", Kind: "page", Status: "published"},
	{Key: "about", Kind: "page", Status: "draft"},
	{Key: "review-article", Kind: "post", Status: "review"},
	{Key: "release-notes", Kind: "post", Status: "scheduled"},
	{Key: "archive-guide", Kind: "page", Status: "archived"},
}

func (s *store) seedContent(
	ctx context.Context,
	seed Config,
	localeIDs map[string]string,
	userIDs map[string]string,
) error {
	if err := s.seedTaxonomies(ctx, seed); err != nil {
		return err
	}
	contentIDs := make(map[string]string, len(demoContent))
	for _, fixture := range demoContent {
		contentID := s.ids.Content(fixture.Key)
		contentIDs[fixture.Key] = contentID
		if err := s.ensure(ctx, "content_items", map[string]any{"account_id": seed.AccountID, "content_key": fixture.Key}, map[string]any{
			"id": contentID, "account_id": seed.AccountID, "kind": fixture.Kind, "content_key": fixture.Key, "status": fixture.Status, "version": int64(1),
		}); err != nil {
			return err
		}
	}

	translationIDs := make(map[string]string)
	for _, item := range demoContent {
		for _, tag := range []string{"th", "en", "ja"} {
			if !translationIncluded(item.Key, tag) {
				continue
			}
			translationID := s.ids.Translation(item.Key, tag)
			translationIDs[item.Key+":"+tag] = translationID
			status := item.Status
			if item.Key == "about" && tag == "ja" {
				status = "draft"
			}
			publishedAt := any(nil)
			if status == "published" {
				publishedAt = seedPublishedAt
			}
			title := contentTitle(item.Key, tag)
			blocks, marshalErr := json.Marshal([]map[string]any{{"type": "text", "data": map[string]any{"text": title + " — local fixture"}}})
			if marshalErr != nil {
				return marshalErr
			}
			values := map[string]any{
				"id": translationID, "account_id": seed.AccountID, "content_id": contentIDs[item.Key], "locale_id": localeIDs[tag],
				"slug": contentSlug(item.Key, tag), "path": contentPath(item.Key, tag), "title": title, "excerpt": "Seeded content for local CMS testing.",
				"blocks": json.RawMessage(blocks), "seo_title": title + " | Go Starter", "seo_description": "Local CMS demo content with SEO, GEO and AEO metadata.",
				"geo_region": "Bangkok", "geo_locality": "Bangkok", "aeo_question": "What is this page?", "aeo_answer": "A local Go starter demo page.",
				"status": status, "version": int64(1), "published_at": publishedAt,
			}
			if err := s.ensure(ctx, "content_translations", map[string]any{"content_id": contentIDs[item.Key], "locale_id": localeIDs[tag]}, values); err != nil {
				return err
			}
			if status == "published" || item.Key == "about" {
				if err := s.seedRevision(ctx, seed, item.Key, tag, contentIDs[item.Key], translationID, userIDs["author"], title, blocks); err != nil {
					return err
				}
			}
		}
	}

	if err := s.seedTaxonomyLinks(ctx, seed, contentIDs); err != nil {
		return err
	}
	if err := s.seedWorkflow(ctx, seed, contentIDs, translationIDs, userIDs); err != nil {
		return err
	}
	if err := s.seedSchedules(ctx, seed, translationIDs["release-notes:en"]); err != nil {
		return err
	}
	if err := s.seedDiscoverability(ctx, seed, localeIDs, contentIDs, translationIDs); err != nil {
		return err
	}
	return nil
}

var seedPublishedAt = time.Date(2026, 1, 15, 9, 0, 0, 0, time.UTC)

func translationIncluded(contentKey, tag string) bool {
	if contentKey == "about" && tag == "ja" {
		return true
	}
	return contentKey == "home" || (contentKey != "home" && tag == "en")
}

func contentTitle(key, tag string) string {
	values := map[string]string{
		"home:th": "หน้าแรก Go Starter", "home:en": "Go Starter Home", "home:ja": "Go Starter ホーム",
		"about:th": "เกี่ยวกับ Go Starter", "about:en": "About Go Starter", "about:ja": "Go Starterについて",
		"review-article:en": "Review workflow article", "release-notes:en": "Scheduled release notes", "archive-guide:en": "Archived guide",
	}
	return values[key+":"+tag]
}

func contentSlug(key, tag string) string {
	if tag == "th" {
		return key + "-th"
	}
	if tag == "ja" {
		return key + "-ja"
	}
	return key
}

func contentPath(key, tag string) string {
	if key == "home" {
		return "/"
	}
	return "/" + tag + "/" + key
}

func (s *store) seedRevision(ctx context.Context, seed Config, key, tag, contentID, translationID, authorID, title string, blocks []byte) error {
	if authorID == "" {
		authorID = s.ids.OwnerUser
	}
	hash := sha256.Sum256([]byte(key + ":" + tag + ":v1"))
	snapshot := json.RawMessage(fmt.Sprintf(`{"title":%q,"blocks":%s}`, title, blocks))
	return s.ensure(ctx, "content_revisions", map[string]any{
		"account_id": seed.AccountID, "content_id": contentID, "translation_id": translationID, "source_version": int64(1),
	}, map[string]any{
		"id": s.ids.Revision(key, tag), "account_id": seed.AccountID, "content_id": contentID, "translation_id": translationID, "author_id": authorID,
		"snapshot_hash": hex.EncodeToString(hash[:]), "change_summary": "Initial local seed snapshot", "source_version": int64(1), "snapshot": snapshot,
	})
}

func (s *store) seedTaxonomies(ctx context.Context, seed Config) error {
	for _, taxonomy := range []struct{ kind, name, slug string }{
		{kind: "category", name: "Documentation", slug: "documentation"},
		{kind: "tag", name: "Starter", slug: "starter"},
	} {
		if err := s.ensure(ctx, "taxonomies", map[string]any{"account_id": seed.AccountID, "kind": taxonomy.kind, "slug": taxonomy.slug}, map[string]any{
			"id": s.ids.Taxonomy(taxonomy.kind, taxonomy.slug), "account_id": seed.AccountID, "kind": taxonomy.kind, "name": taxonomy.name, "slug": taxonomy.slug, "version": int64(1),
		}); err != nil {
			return err
		}
	}
	return nil
}

func (s *store) seedTaxonomyLinks(ctx context.Context, seed Config, contentIDs map[string]string) error {
	for _, key := range []string{"home", "about", "review-article"} {
		for _, taxonomy := range []struct{ kind, slug string }{{kind: "category", slug: "documentation"}, {kind: "tag", slug: "starter"}} {
			if err := s.link(ctx, "content_taxonomy_terms", map[string]any{
				"account_id": seed.AccountID, "content_id": contentIDs[key], "taxonomy_id": s.ids.Taxonomy(taxonomy.kind, taxonomy.slug),
			}); err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *store) seedWorkflow(ctx context.Context, seed Config, contentIDs, translationIDs map[string]string, userIDs map[string]string) error {
	events := []struct{ key, tag, from, to string }{
		{key: "home", tag: "en", from: "draft", to: "published"},
		{key: "review-article", tag: "en", from: "draft", to: "review"},
	}
	for _, event := range events {
		if err := s.ensure(ctx, "workflow_events", map[string]any{"id": s.ids.Record("workflow", event.key+":"+event.tag)}, map[string]any{
			"id": s.ids.Record("workflow", event.key+":"+event.tag), "account_id": seed.AccountID, "content_id": contentIDs[event.key], "translation_id": translationIDs[event.key+":"+event.tag],
			"actor_id": userIDs["owner"], "from_status": event.from, "to_status": event.to, "request_id": "seed-workflow-" + event.key, "occurred_at": seedPublishedAt,
		}); err != nil {
			return err
		}
	}
	return nil
}

func (s *store) seedSchedules(ctx context.Context, seed Config, translationID string) error {
	if translationID == "" {
		return fmt.Errorf("seed scheduled translation is missing")
	}
	return s.ensure(ctx, "publication_schedules", map[string]any{"account_id": seed.AccountID, "idempotency_key": "seed-release-notes"}, map[string]any{
		"id": s.ids.Record("schedule", "release-notes"), "account_id": seed.AccountID, "translation_id": translationID, "publish_at": time.Date(2030, 1, 1, 9, 0, 0, 0, time.UTC),
		"expected_version": int64(1), "idempotency_key": "seed-release-notes", "state": "pending", "attempts": 0, "failure_reason": "",
	})
}
