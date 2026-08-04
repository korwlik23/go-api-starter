package domain

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestNotificationMarkReadIsIdempotent(t *testing.T) {
	now := time.Date(2026, 8, 4, 12, 0, 0, 0, time.UTC)
	notification, err := New("notification-1", "user-1", "System alert", "Backup completed", "warning", now)
	require.NoError(t, err)
	require.False(t, notification.IsRead())

	require.NoError(t, notification.MarkRead(now))
	require.True(t, notification.IsRead())
	require.Equal(t, now, *notification.ReadAt)

	later := now.Add(time.Minute)
	require.NoError(t, notification.MarkRead(later))
	require.Equal(t, now, *notification.ReadAt)
}

func TestNotificationRejectsMissingOwnershipAndContent(t *testing.T) {
	_, err := New("notification-1", "", "title", "body", "info", time.Now())
	require.ErrorIs(t, err, ErrInvalidNotification)
	_, err = New("notification-1", "user-1", "", "body", "info", time.Now())
	require.ErrorIs(t, err, ErrInvalidNotification)
}
