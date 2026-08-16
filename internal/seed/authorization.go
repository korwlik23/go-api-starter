package seed

import (
	"context"
	"fmt"

	"github.com/korwlik23/go-api-starter/internal/platform/databaseid"
)

type permissionRow struct {
	ID     databaseid.ID `gorm:"column:id"`
	Key    string        `gorm:"column:permission_key"`
	Scope  string        `gorm:"column:scope"`
	Active bool          `gorm:"column:active"`
}

func (s *store) seedAuthorization(
	ctx context.Context,
	seed Config,
	userIDs map[string]string,
) error {
	for _, fixture := range demoRoles {
		userID := userIDs[fixture.Key]
		if userID == "" {
			return fmt.Errorf("seed user %q is missing", fixture.Key)
		}
		accountRoleID := s.ids.Role(fixture.Key)
		if fixture.Key == "owner" {
			accountRoleID = s.ids.Role("owner")
		}
		if err := s.ensure(ctx, "roles", map[string]any{"id": accountRoleID}, map[string]any{
			"id": accountRoleID, "owner_type": "account", "account_id": seed.AccountID, "name": fixture.Name, "version": int64(1),
		}); err != nil {
			return err
		}
		membershipID, err := s.findID(ctx, "memberships", map[string]any{"account_id": seed.AccountID, "user_id": userID})
		if err != nil {
			return fmt.Errorf("seed membership for %q: %w", fixture.Key, err)
		}
		if err := s.ensure(ctx, "membership_roles", map[string]any{"account_id": seed.AccountID, "membership_id": membershipID, "role_id": accountRoleID}, map[string]any{
			"account_id": seed.AccountID, "membership_id": membershipID, "role_id": accountRoleID, "assignment_id": s.ids.Assignment("account/" + fixture.Key),
		}); err != nil {
			return err
		}
		for _, key := range fixture.AccountKeys {
			if err := s.grantPermission(ctx, accountRoleID, key); err != nil {
				return fmt.Errorf("seed account permission %q: %w", key, err)
			}
		}
		if len(fixture.SystemKeys) == 0 {
			continue
		}
		systemRoleID := s.ids.SystemRole(fixture.Key)
		if err := s.ensure(ctx, "roles", map[string]any{"id": systemRoleID}, map[string]any{
			"id": systemRoleID, "owner_type": "system", "account_id": nil, "name": fixture.Name + " System", "version": int64(1),
		}); err != nil {
			return err
		}
		if err := s.ensure(ctx, "system_role_assignments", map[string]any{"user_id": userID, "role_id": systemRoleID}, map[string]any{
			"user_id": userID, "role_owner_type": "system", "role_id": systemRoleID, "assignment_id": s.ids.Assignment("system/" + fixture.Key),
		}); err != nil {
			return err
		}
		for _, key := range fixture.SystemKeys {
			if err := s.grantPermission(ctx, systemRoleID, key); err != nil {
				return fmt.Errorf("seed system permission %q: %w", key, err)
			}
		}
	}
	return nil
}

func (s *store) grantPermission(ctx context.Context, roleID, key string) error {
	var permission permissionRow
	where, err := s.encodeMap(map[string]any{"permission_key": key})
	if err != nil {
		return err
	}
	result := s.db.WithContext(ctx).Table("permissions").Where(where).Take(&permission)
	if result.Error != nil {
		return result.Error
	}
	if !permission.Active || permission.ID.String() == "" {
		return fmt.Errorf("permission %q is inactive or missing", key)
	}
	return s.link(ctx, "role_permissions", map[string]any{
		"role_id": roleID, "permission_id": permission.ID.String(),
	})
}
