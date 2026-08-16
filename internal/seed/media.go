package seed

import "context"

func (s *store) seedMedia(ctx context.Context, seed Config) error {
	return s.ensure(ctx, "media_assets", map[string]any{"account_id": seed.AccountID, "storage_key": "seed-go-starter.png"}, map[string]any{
		"id": s.ids.Record("media", "go-starter"), "account_id": seed.AccountID, "storage_key": "seed-go-starter.png", "original_name": "go-starter.png",
		"mime_type": "image/png", "byte_size": int64(68), "sha256": "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef", "width": int32(256), "height": int32(256), "version": int64(1),
	})
}
