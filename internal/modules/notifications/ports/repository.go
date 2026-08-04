package ports

import (
	"context"
	"github.com/korwlik23/go-api-starter/internal/modules/notifications/domain"
)

var ErrNotFound = domain.ErrInvalidNotification

type Repository interface {
	ListForUser(context.Context, string, int) ([]domain.Notification, error)
	FindForUser(context.Context, string, string) (domain.Notification, error)
	MarkRead(context.Context, string, string, domain.Notification) error
	MarkAllRead(context.Context, string, domain.Notification) error
}
