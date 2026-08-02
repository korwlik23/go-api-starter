package gorm

import (
	"context"
	"errors"

	"github.com/korwlik23/go-api-starter/internal/platform/database"
	gormdb "gorm.io/gorm"
)

var ErrInvalidContentRepository = errors.New("invalid content repository")

// ContentRepository owns the database connection boundary for publishing
// content queries and mutations. Query and record mapping details live in
// separate files so each concern remains independently reviewable.
type ContentRepository struct {
	db          *gormdb.DB
	connections database.ConnectionProvider
}

func NewContentRepository(db *gormdb.DB) (*ContentRepository, error) {
	if db == nil {
		return nil, ErrInvalidContentRepository
	}
	manager, err := database.NewTransactionManager(db)
	if err != nil {
		return nil, ErrInvalidContentRepository
	}
	return NewContentRepositoryWithDependencies(db, manager)
}

// NewContentRepositoryWithDependencies lets publishing use a shared
// transaction boundary for content, workflow, revisions, and audit.
func NewContentRepositoryWithDependencies(
	db *gormdb.DB,
	connections database.ConnectionProvider,
) (*ContentRepository, error) {
	if db == nil || connections == nil {
		return nil, ErrInvalidContentRepository
	}
	return &ContentRepository{db: db, connections: connections}, nil
}

func (repository *ContentRepository) connection(ctx context.Context) (*gormdb.DB, error) {
	if repository == nil || repository.connections == nil || ctx == nil {
		return nil, ErrInvalidContentRepository
	}
	return repository.connections.Connection(ctx)
}
