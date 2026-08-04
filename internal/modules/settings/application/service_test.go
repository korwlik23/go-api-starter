package application

import (
	"context"
	"testing"

	authdomain "github.com/korwlik23/go-api-starter/internal/modules/authorization/domain"
	"github.com/korwlik23/go-api-starter/internal/modules/settings/domain"
	"github.com/korwlik23/go-api-starter/internal/modules/settings/ports"
	"github.com/stretchr/testify/require"
)

type repositoryFake struct {
	item   domain.Setting
	exists bool
	saved  []domain.Setting
}

func (fake *repositoryFake) List(context.Context) ([]domain.Setting, error) {
	if !fake.exists {
		return nil, nil
	}
	return []domain.Setting{fake.item}, nil
}
func (fake *repositoryFake) Find(context.Context, string) (domain.Setting, error) {
	if !fake.exists {
		return domain.Setting{}, ports.ErrNotFound
	}
	return fake.item, nil
}
func (fake *repositoryFake) Save(_ context.Context, item domain.Setting, expected int64) error {
	if expected > 0 && fake.item.Version != expected {
		return domain.ErrVersionConflict
	}
	fake.item = item
	fake.exists = true
	fake.saved = append(fake.saved, item)
	return nil
}

type permissionAuthorizerFake struct{ allowed bool }

func (fake permissionAuthorizerFake) Allowed(context.Context, string, authdomain.PermissionKey) (bool, error) {
	return fake.allowed, nil
}

type auditFake struct{ events []SettingAuditEvent }

func (fake *auditFake) RecordSettingChange(_ context.Context, event SettingAuditEvent) error {
	fake.events = append(fake.events, event)
	return nil
}

func TestServiceUpdatesSettingsWithOptimisticVersionAndAudit(t *testing.T) {
	repository := &repositoryFake{item: domain.Setting{Key: "app.name", Value: "Starter", Version: 1}, exists: true}
	auditor := &auditFake{}
	service, err := NewService(permissionAuthorizerFake{allowed: true}, repository, nil, auditor)
	require.NoError(t, err)
	item, err := service.Update(context.Background(), "user-1", "app.name", "Go Starter", "req-1", 1)
	require.NoError(t, err)
	require.Equal(t, int64(2), item.Version)
	require.Equal(t, "Go Starter", item.Value)
	require.Len(t, auditor.events, 1)
	require.Equal(t, "settings.updated", auditor.events[0].Action)
	_, err = service.Update(context.Background(), "user-1", "app.name", "stale", "req-2", 1)
	require.ErrorIs(t, err, domain.ErrVersionConflict)
}

func TestServiceRejectsDeniedActor(t *testing.T) {
	service, err := NewService(permissionAuthorizerFake{allowed: false}, &repositoryFake{}, nil, nil)
	require.NoError(t, err)
	_, err = service.List(context.Background(), "user-1")
	require.ErrorIs(t, err, ErrDenied)
}
