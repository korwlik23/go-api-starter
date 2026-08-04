package domain

import (
	"errors"
	"strings"
	"time"
)

var ErrInvalidNotification = errors.New("notifications: invalid notification")

type Notification struct {
	ID        string
	UserID    string
	Title     string
	Body      string
	Kind      string
	ReadAt    *time.Time
	CreatedAt time.Time
}

func New(id, userID, title, body, kind string, createdAt time.Time) (Notification, error) {
	if strings.TrimSpace(id) == "" || strings.TrimSpace(userID) == "" ||
		strings.TrimSpace(title) == "" || strings.TrimSpace(body) == "" ||
		createdAt.IsZero() || len(title) > 255 || len(body) > 16_384 {
		return Notification{}, ErrInvalidNotification
	}
	if strings.TrimSpace(kind) == "" {
		kind = "info"
	}
	return Notification{
		ID: id, UserID: userID, Title: strings.TrimSpace(title),
		Body: body, Kind: strings.TrimSpace(kind), CreatedAt: createdAt.UTC(),
	}, nil
}

func (notification *Notification) MarkRead(at time.Time) error {
	if notification == nil || notification.ID == "" || at.IsZero() {
		return ErrInvalidNotification
	}
	if notification.ReadAt == nil {
		value := at.UTC()
		notification.ReadAt = &value
	}
	return nil
}

func (notification Notification) IsRead() bool { return notification.ReadAt != nil }
