package seed

import "context"

func (s *store) seedNavigation(ctx context.Context, seed Config, localeIDs map[string]string) error {
	for _, tag := range []string{"th", "en"} {
		menuID := s.ids.Menu(tag)
		if err := s.ensure(ctx, "navigation_menus", map[string]any{"account_id": seed.AccountID, "locale_id": localeIDs[tag], "menu_key": "primary"}, map[string]any{
			"id": menuID, "account_id": seed.AccountID, "locale_id": localeIDs[tag], "menu_key": "primary", "name": "Primary Navigation", "enabled": true, "version": int64(1),
		}); err != nil {
			return err
		}
		items := []struct {
			key, label, target string
			position           int64
		}{
			{key: "home", label: "Home", target: "/", position: 0},
			{key: "about", label: "About", target: "/" + tag + "/about", position: 1},
		}
		for _, item := range items {
			label := item.label
			if tag == "th" {
				if item.key == "home" {
					label = "หน้าแรก"
				} else {
					label = "เกี่ยวกับเรา"
				}
			}
			if err := s.ensure(ctx, "navigation_menu_items", map[string]any{"account_id": seed.AccountID, "menu_id": menuID, "id": s.ids.MenuItem(tag, item.key)}, map[string]any{
				"id": s.ids.MenuItem(tag, item.key), "account_id": seed.AccountID, "menu_id": menuID, "parent_id": nil, "label": label, "target_kind": "internal_path", "target": item.target,
				"position": item.position, "enabled": true, "version": int64(1),
			}); err != nil {
				return err
			}
		}
	}
	return nil
}
