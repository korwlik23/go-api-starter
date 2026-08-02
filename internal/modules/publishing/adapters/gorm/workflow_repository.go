package gorm

import (
	"context"
	"fmt"

	publishingdomain "github.com/korwlik23/go-api-starter/internal/modules/publishing/domain"
	"github.com/korwlik23/go-api-starter/internal/modules/publishing/ports"
	"github.com/korwlik23/go-api-starter/internal/platform/database"
	"github.com/korwlik23/go-api-starter/internal/platform/databaseid"
	gormdb "gorm.io/gorm"
)

func NewWorkflowRepository(db *gormdb.DB) (*WorkflowRepository, error) {
	if db == nil {
		return nil, ErrInvalidContentRepository
	}
	manager, err := database.NewTransactionManager(db)
	if err != nil {
		return nil, ErrInvalidContentRepository
	}
	return NewWorkflowRepositoryWithDependencies(db, manager)
}

func NewWorkflowRepositoryWithDependencies(
	db *gormdb.DB,
	connections database.ConnectionProvider,
) (*WorkflowRepository, error) {
	if db == nil || connections == nil {
		return nil, ErrInvalidContentRepository
	}
	content, err := NewContentRepositoryWithDependencies(db, connections)
	if err != nil {
		return nil, err
	}
	return &WorkflowRepository{
		db: db, connections: connections,
		content: content,
	}, nil
}

type WorkflowRepository struct {
	db          *gormdb.DB
	connections database.ConnectionProvider
	content     *ContentRepository
}

func (repository *WorkflowRepository) connection(ctx context.Context) (*gormdb.DB, error) {
	if repository == nil || repository.connections == nil || ctx == nil {
		return nil, ErrInvalidContentRepository
	}
	return repository.connections.Connection(ctx)
}

func (repository *WorkflowRepository) Get(
	ctx context.Context,
	accountID string,
	contentID string,
	localeID string,
) (ports.ContentDetail, error) {
	if repository == nil || repository.content == nil {
		return ports.ContentDetail{}, ErrInvalidContentRepository
	}
	detail, err := repository.content.Get(ctx, accountID, contentID, localeID)
	if err != nil {
		if err == ports.ErrContentNotFound {
			return ports.ContentDetail{}, ports.ErrWorkflowNotFound
		}
		return ports.ContentDetail{}, err
	}
	return detail, nil
}

func (repository *WorkflowRepository) Transition(
	ctx context.Context,
	command ports.WorkflowTransitionCommand,
) (ports.ContentDetail, error) {
	db, err := repository.connection(ctx)
	if err != nil {
		return ports.ContentDetail{}, err
	}
	dialect := db.Dialector.Name()
	accountID, err := databaseid.Encode(dialect, command.AccountID)
	if err != nil {
		return ports.ContentDetail{}, ports.ErrWorkflowNotFound
	}
	contentID, err := databaseid.Encode(dialect, command.ContentID)
	if err != nil {
		return ports.ContentDetail{}, ports.ErrWorkflowNotFound
	}
	translationID, err := databaseid.Encode(dialect, command.TranslationID)
	if err != nil {
		return ports.ContentDetail{}, ports.ErrWorkflowNotFound
	}
	updatedAt := command.UpdatedAt.UTC()
	if updatedAt.IsZero() || publishingdomain.ValidateTransition(command.FromStatus, command.ToStatus) != nil {
		return ports.ContentDetail{}, ports.ErrInvalidWorkflowRepository
	}
	publishedAt := any(nil)
	if command.ToStatus == publishingdomain.StatusPublished {
		publishedAt = updatedAt
	}
	translationResult := db.Table("content_translations").Where(
		"account_id = ? AND content_id = ? AND id = ? AND status = ? AND version = ? AND deleted_at IS NULL",
		accountID, contentID, translationID, command.FromStatus, command.ExpectedVersion,
	).Updates(map[string]any{
		"status": command.ToStatus, "published_at": publishedAt,
		"version": command.ExpectedVersion + 1, "updated_at": updatedAt,
	})
	if translationResult.Error != nil {
		return ports.ContentDetail{}, fmt.Errorf("transition content translation: %w", translationResult.Error)
	}
	if translationResult.RowsAffected != 1 {
		return ports.ContentDetail{}, ports.ErrWorkflowConflict
	}
	itemResult := db.Table("content_items").Where(
		"account_id = ? AND id = ? AND status = ? AND deleted_at IS NULL",
		accountID, contentID, command.FromStatus,
	).Updates(map[string]any{
		"status": command.ToStatus, "version": gormdb.Expr("version + ?", 1),
		"updated_at": updatedAt,
	})
	if itemResult.Error != nil {
		return ports.ContentDetail{}, fmt.Errorf("transition content item: %w", itemResult.Error)
	}
	if itemResult.RowsAffected != 1 {
		return ports.ContentDetail{}, ports.ErrWorkflowConflict
	}
	return repository.content.Get(ctx, command.AccountID, command.ContentID, command.LocaleID)
}

func (repository *WorkflowRepository) AppendEvent(
	ctx context.Context,
	event ports.WorkflowEvent,
) error {
	if err := ports.ValidateWorkflowEvent(event); err != nil {
		return err
	}
	db, err := repository.connection(ctx)
	if err != nil {
		return err
	}
	dialect := db.Dialector.Name()
	id, err := databaseid.Encode(dialect, event.ID)
	if err != nil {
		return ports.ErrInvalidWorkflowRepository
	}
	accountID, err := databaseid.Encode(dialect, event.AccountID)
	if err != nil {
		return ports.ErrInvalidWorkflowRepository
	}
	contentID, err := databaseid.Encode(dialect, event.ContentID)
	if err != nil {
		return ports.ErrInvalidWorkflowRepository
	}
	translationID, err := databaseid.Encode(dialect, event.TranslationID)
	if err != nil {
		return ports.ErrInvalidWorkflowRepository
	}
	actorID, err := databaseid.Encode(dialect, event.ActorID)
	if err != nil {
		return ports.ErrInvalidWorkflowRepository
	}
	if err := db.Table("workflow_events").Create(map[string]any{
		"id": id, "account_id": accountID, "content_id": contentID,
		"translation_id": translationID, "actor_id": actorID,
		"from_status": event.FromStatus, "to_status": event.ToStatus,
		"request_id": event.RequestID, "occurred_at": event.OccurredAt.UTC(),
	}).Error; err != nil {
		return fmt.Errorf("append workflow event: %w", err)
	}
	return nil
}

var _ ports.WorkflowRepository = (*WorkflowRepository)(nil)
