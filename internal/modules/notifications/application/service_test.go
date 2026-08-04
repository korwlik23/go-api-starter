package application

import (
	"context"
	"testing"
	"time"

	authapp "github.com/korwlik23/go-api-starter/internal/modules/authorization/application"
	"github.com/korwlik23/go-api-starter/internal/modules/notifications/domain"
	"github.com/korwlik23/go-api-starter/internal/modules/notifications/ports"
	"github.com/stretchr/testify/require"
)

type evaluatorFake struct{ allowed bool }

func (fake evaluatorFake) Execute(context.Context, authapp.PermissionRequest) (bool, error) {
	return fake.allowed, nil
}

type notificationRepositoryFake struct {
	item   domain.Notification
	marked bool
	all    bool
}

func (fake *notificationRepositoryFake) ListForUser(context.Context, string, int) ([]domain.Notification, error) {
	return []domain.Notification{fake.item}, nil
}
func (fake *notificationRepositoryFake) FindForUser(context.Context, string, string) (domain.Notification, error) {
	if fake.item.ID == "" {
		return domain.Notification{}, ports.ErrNotFound
	}
	return fake.item, nil
}
func (fake *notificationRepositoryFake) MarkRead(context.Context, string, string, domain.Notification) error {
	fake.marked = true
	return nil
}
func (fake *notificationRepositoryFake) MarkAllRead(context.Context, string, domain.Notification) error {
	fake.all = true
	return nil
}

func TestServiceMarksOnlyCurrentUsersNotificationRead(t *testing.T) {
	created := time.Now().UTC()
	item, err := domain.New("00000000-0000-0000-0000-000000000001", "00000000-0000-0000-0000-000000000002", "Title", "Body", "info", created)
	require.NoError(t, err)
	repository := &notificationRepositoryFake{item: item}
	service, err := NewService(evaluatorFake{allowed: true}, repository, nil)
	require.NoError(t, err)
	require.NoError(t, service.MarkRead(context.Background(), item.UserID, item.ID))
	require.True(t, repository.marked)
}

func TestServiceDeniesNotificationAccessWithoutPermission(t *testing.T) {
	service, err := NewService(evaluatorFake{allowed: false}, &notificationRepositoryFake{}, nil)
	require.NoError(t, err)
	_, err = service.List(context.Background(), "00000000-0000-0000-0000-000000000002", 20)
	require.ErrorIs(t, err, ErrDenied)
}
