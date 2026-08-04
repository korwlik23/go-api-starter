package gorm

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/korwlik23/go-api-starter/internal/modules/notifications/domain"
	"github.com/korwlik23/go-api-starter/internal/modules/notifications/ports"
	"github.com/korwlik23/go-api-starter/internal/platform/database"
	"github.com/korwlik23/go-api-starter/internal/platform/databaseid"
	gormdb "gorm.io/gorm"
)

var ErrInvalidRepository = errors.New("notifications: invalid repository")

type Repository struct{ connections database.ConnectionProvider }

type notificationRecord struct {
	ID        databaseid.ID `gorm:"column:id"`
	UserID    databaseid.ID `gorm:"column:user_id"`
	Title     string        `gorm:"column:title"`
	Body      string        `gorm:"column:body"`
	Kind      string        `gorm:"column:kind"`
	ReadAt    *time.Time    `gorm:"column:read_at"`
	CreatedAt time.Time     `gorm:"column:created_at"`
}

func (notificationRecord) TableName() string { return "notifications" }

func NewRepositoryWithDependencies(connections database.ConnectionProvider) (*Repository, error) {
	if connections == nil {
		return nil, ErrInvalidRepository
	}
	return &Repository{connections: connections}, nil
}

func (repository *Repository) connection(ctx context.Context) (*gormdb.DB, error) {
	if repository == nil || repository.connections == nil || ctx == nil {
		return nil, ErrInvalidRepository
	}
	return repository.connections.Connection(ctx)
}

func (repository *Repository) ListForUser(ctx context.Context, userID string, limit int) ([]domain.Notification, error) {
	db, err := repository.connection(ctx)
	if err != nil {
		return nil, err
	}
	encoded, err := databaseid.Encode(db.Dialector.Name(), userID)
	if err != nil {
		return nil, err
	}
	if limit < 1 || limit > 100 {
		limit = 50
	}
	var records []notificationRecord
	if err := db.Where("user_id = ?", encoded).Order("created_at DESC, id DESC").Limit(limit).Find(&records).Error; err != nil {
		return nil, fmt.Errorf("list notifications: %w", err)
	}
	items := make([]domain.Notification, 0, len(records))
	for _, record := range records {
		item, mapErr := fromRecord(record)
		if mapErr != nil {
			return nil, mapErr
		}
		items = append(items, item)
	}
	return items, nil
}

func (repository *Repository) FindForUser(ctx context.Context, userID, notificationID string) (domain.Notification, error) {
	db, err := repository.connection(ctx)
	if err != nil {
		return domain.Notification{}, err
	}
	encodedUser, err := databaseid.Encode(db.Dialector.Name(), userID)
	if err != nil {
		return domain.Notification{}, err
	}
	encodedID, err := databaseid.Encode(db.Dialector.Name(), notificationID)
	if err != nil {
		return domain.Notification{}, err
	}
	var record notificationRecord
	if err := db.Where("id = ? AND user_id = ?", encodedID, encodedUser).First(&record).Error; err != nil {
		if errors.Is(err, gormdb.ErrRecordNotFound) {
			return domain.Notification{}, ports.ErrNotFound
		}
		return domain.Notification{}, fmt.Errorf("find notification: %w", err)
	}
	return fromRecord(record)
}

func (repository *Repository) MarkRead(ctx context.Context, userID, notificationID string, notification domain.Notification) error {
	db, err := repository.connection(ctx)
	if err != nil {
		return err
	}
	encodedUser, err := databaseid.Encode(db.Dialector.Name(), userID)
	if err != nil {
		return err
	}
	encodedID, err := databaseid.Encode(db.Dialector.Name(), notificationID)
	if err != nil {
		return err
	}
	if notification.ReadAt == nil {
		return domain.ErrInvalidNotification
	}
	result := db.Model(&notificationRecord{}).Where("id = ? AND user_id = ? AND read_at IS NULL", encodedID, encodedUser).Update("read_at", *notification.ReadAt)
	if result.Error != nil {
		return fmt.Errorf("mark notification read: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return ports.ErrNotFound
	}
	return nil
}

func (repository *Repository) MarkAllRead(ctx context.Context, userID string, notification domain.Notification) error {
	db, err := repository.connection(ctx)
	if err != nil {
		return err
	}
	encodedUser, err := databaseid.Encode(db.Dialector.Name(), userID)
	if err != nil {
		return err
	}
	if notification.ReadAt == nil {
		return domain.ErrInvalidNotification
	}
	if err := db.Model(&notificationRecord{}).Where("user_id = ? AND read_at IS NULL", encodedUser).Update("read_at", *notification.ReadAt).Error; err != nil {
		return fmt.Errorf("mark all notifications read: %w", err)
	}
	return nil
}

func fromRecord(record notificationRecord) (domain.Notification, error) {
	item, err := domain.New(record.ID.String(), record.UserID.String(), record.Title, record.Body, record.Kind, record.CreatedAt)
	if err != nil {
		return domain.Notification{}, err
	}
	item.ReadAt = record.ReadAt
	return item, nil
}

var _ ports.Repository = (*Repository)(nil)
