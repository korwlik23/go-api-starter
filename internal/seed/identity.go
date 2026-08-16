package seed

import (
	"context"
	"fmt"
	"strings"

	"github.com/korwlik23/go-api-starter/internal/platform/crypto"
	"github.com/korwlik23/go-api-starter/internal/platform/databaseid"
)

func (s *store) ensureFoundation(ctx context.Context) (string, error) {
	if _, err := s.findID(ctx, "accounts", map[string]any{"id": s.ids.Account}); err != nil {
		return "", fmt.Errorf("seed foundation is missing: %w", err)
	}
	var row struct {
		UserID databaseid.ID `gorm:"column:user_id"`
	}
	query, err := s.where(
		s.db.WithContext(ctx).Table("memberships").Select("user_id"),
		map[string]any{"account_id": s.ids.Account, "status": "active"},
	)
	if err != nil {
		return "", err
	}
	result := query.Order("created_at ASC").Take(&row)
	if result.Error != nil {
		return "", fmt.Errorf("seed owner membership is missing: %w", result.Error)
	}
	return row.UserID.String(), nil
}

func (s *store) seedIdentities(
	ctx context.Context,
	seed Config,
) (map[string]string, error) {
	ownerID, err := s.ensureFoundation(ctx)
	if err != nil {
		return nil, err
	}
	ids := map[string]string{"owner": ownerID}
	hasher, err := crypto.NewPasswordHasher(crypto.DefaultPasswordPolicy())
	if err != nil {
		return nil, err
	}
	if len(seed.Password) < 12 || len(seed.Password) > 128 {
		return nil, fmt.Errorf("seed: password must be between 12 and 128 characters")
	}

	for _, fixture := range demoRoles {
		userID := ownerID
		if fixture.Key != "owner" {
			userID = s.ids.User(fixture.Key)
			if existing, findErr := s.findID(ctx, "users", map[string]any{"normalized_email": strings.ToLower(fixture.Email)}); findErr == nil && existing != userID {
				return nil, fmt.Errorf("seed user email %q belongs to a different ID", fixture.Email)
			} else if findErr != nil && !isNotFound(findErr) {
				return nil, findErr
			}
			hash, hashErr := hasher.Hash(seed.Password)
			if hashErr != nil {
				return nil, hashErr
			}
			if err := s.ensure(ctx, "users", map[string]any{"id": userID}, map[string]any{
				"id": userID, "normalized_email": strings.ToLower(fixture.Email), "status": fixture.Status, "authorization_version": int64(1),
			}); err != nil {
				return nil, err
			}
			if err := s.ensure(ctx, "password_credentials", map[string]any{"user_id": userID}, map[string]any{
				"user_id": userID, "password_hash": hash,
			}); err != nil {
				return nil, err
			}
			if err := s.ensure(ctx, "memberships", map[string]any{"account_id": seed.AccountID, "user_id": userID}, map[string]any{
				"id": s.ids.Membership(fixture.Key), "account_id": seed.AccountID, "user_id": userID, "status": fixture.Status, "authorization_version": int64(1),
			}); err != nil {
				return nil, err
			}
		}
		if err := s.ensure(ctx, "user_profiles", map[string]any{"user_id": userID}, map[string]any{
			"user_id": userID, "display_name": fixture.DisplayName, "timezone": "Asia/Bangkok", "preferred_locale": fixture.Locale, "version": int64(1),
		}); err != nil {
			return nil, err
		}
		ids[fixture.Key] = userID
	}
	return ids, nil
}

func isNotFound(err error) bool {
	return err != nil && strings.Contains(strings.ToLower(err.Error()), "record not found")
}
