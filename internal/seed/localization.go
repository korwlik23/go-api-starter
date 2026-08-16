package seed

import (
	"context"
	"fmt"
	"strings"

	"github.com/korwlik23/go-api-starter/internal/platform/databaseid"
)

type localeRow struct {
	ID  databaseid.ID `gorm:"column:id"`
	Tag string        `gorm:"column:tag"`
}

func (s *store) seedLocalization(ctx context.Context) (map[string]string, error) {
	localeIDs := make(map[string]string, 3)
	localeDefinitions := []struct {
		tag, name, fallback string
		enabled, selectable bool
		defaultSlot         any
	}{
		{tag: "th", name: "ไทย", enabled: true, selectable: true, defaultSlot: int16(1)},
		{tag: "en", name: "English", enabled: true, selectable: true, defaultSlot: nil},
		{tag: "ja", name: "日本語", fallback: "en", enabled: true, selectable: false, defaultSlot: nil},
	}
	for _, definition := range localeDefinitions {
		var row localeRow
		result := s.db.WithContext(ctx).Table("locales").Select("id", "tag").Where("tag = ?", definition.tag).Take(&row)
		if isNotFound(result.Error) {
			values := map[string]any{
				"id": s.ids.Locale(definition.tag), "tag": definition.tag, "name": definition.name, "direction": "ltr",
				"enabled": definition.enabled, "user_selectable": definition.selectable, "default_slot": definition.defaultSlot, "version": int64(1),
			}
			if definition.fallback != "" {
				values["fallback_locale_id"] = localeIDs[definition.fallback]
			}
			if err := s.ensure(ctx, "locales", map[string]any{"tag": definition.tag}, values); err != nil {
				return nil, err
			}
			row.ID = databaseid.ID(s.ids.Locale(definition.tag))
		} else if result.Error != nil {
			return nil, result.Error
		} else {
			values := map[string]any{
				"name": definition.name, "direction": "ltr", "enabled": definition.enabled,
				"user_selectable": definition.selectable, "default_slot": definition.defaultSlot, "version": int64(1),
			}
			if definition.fallback != "" {
				values["fallback_locale_id"] = localeIDs[definition.fallback]
			}
			if err := s.ensure(ctx, "locales", map[string]any{"tag": definition.tag}, values); err != nil {
				return nil, err
			}
		}
		localeIDs[definition.tag] = row.ID.String()
	}
	if localeIDs["ja"] == "" || localeIDs["en"] == "" {
		return nil, fmt.Errorf("seed locale IDs are incomplete")
	}

	translations := map[string]map[string]string{
		"common": {
			"app_name": "Go Starter",
			"save":     "Save",
			"cancel":   "Cancel",
		},
		"navigation": {
			"home":    "Home",
			"about":   "About",
			"contact": "Contact",
		},
		"content": {
			"home_title":  "Welcome to the Go Starter",
			"about_title": "About this starter",
		},
	}
	for tag, localeID := range localeIDs {
		for category, values := range translations {
			for key, value := range values {
				translated := value
				switch tag {
				case "th":
					translated = thaiTranslation(category, key)
				case "ja":
					translated = japaneseTranslation(category, key)
				}
				if strings.TrimSpace(translated) == "" {
					translated = value
				}
				if err := s.ensure(ctx, "translations", map[string]any{
					"locale_id": localeID, "category": category, "translation_key": key,
				}, map[string]any{
					"locale_id": localeID, "category": category, "translation_key": key, "value": translated, "source": "editor", "version": int64(1),
				}); err != nil {
					return nil, err
				}
			}
		}
	}
	return localeIDs, nil
}

func thaiTranslation(category, key string) string {
	values := map[string]string{"common.save": "บันทึก", "common.cancel": "ยกเลิก", "navigation.home": "หน้าแรก", "navigation.about": "เกี่ยวกับเรา", "navigation.contact": "ติดต่อเรา", "content.home_title": "ยินดีต้อนรับสู่ Go Starter", "content.about_title": "เกี่ยวกับสตาร์เตอร์นี้"}
	return values[category+"."+key]
}

func japaneseTranslation(category, key string) string {
	values := map[string]string{"common.save": "保存", "common.cancel": "キャンセル", "navigation.home": "ホーム", "navigation.about": "概要", "navigation.contact": "お問い合わせ", "content.home_title": "Go Starterへようこそ", "content.about_title": "このスターターについて"}
	return values[category+"."+key]
}
