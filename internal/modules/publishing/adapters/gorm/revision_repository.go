package gorm

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	publishingdomain "github.com/korwlik23/go-api-starter/internal/modules/publishing/domain"
	"github.com/korwlik23/go-api-starter/internal/modules/publishing/ports"
	"github.com/korwlik23/go-api-starter/internal/platform/database"
	"github.com/korwlik23/go-api-starter/internal/platform/databaseid"
	gormdb "gorm.io/gorm"
)

var ErrInvalidRevisionRepository = errors.New("invalid revision repository")

type RevisionRepository struct {
	db          *gormdb.DB
	connections database.ConnectionProvider
}

type revisionRecord struct {
	RevisionID    databaseid.ID `gorm:"column:revision_id"`
	AccountID     databaseid.ID `gorm:"column:account_id"`
	ContentID     databaseid.ID `gorm:"column:content_id"`
	TranslationID databaseid.ID `gorm:"column:translation_id"`
	AuthorID      databaseid.ID `gorm:"column:author_id"`
	SnapshotHash  string        `gorm:"column:snapshot_hash"`
	ChangeSummary string        `gorm:"column:change_summary"`
	SourceVersion int64         `gorm:"column:source_version"`
	Snapshot      []byte        `gorm:"column:snapshot"`
	CreatedAt     time.Time     `gorm:"column:created_at"`
}

func NewRevisionRepository(db *gormdb.DB) (*RevisionRepository, error) {
	if db == nil {
		return nil, ErrInvalidRevisionRepository
	}
	manager, err := database.NewTransactionManager(db)
	if err != nil {
		return nil, ErrInvalidRevisionRepository
	}
	return &RevisionRepository{db: db, connections: manager}, nil
}

func (repository *RevisionRepository) connection(ctx context.Context) (*gormdb.DB, error) {
	if repository == nil || repository.connections == nil || ctx == nil {
		return nil, ErrInvalidRevisionRepository
	}
	return repository.connections.Connection(ctx)
}

func (repository *RevisionRepository) Create(
	ctx context.Context,
	command ports.CreateRevisionCommand,
) error {
	db, err := repository.connection(ctx)
	if err != nil {
		return err
	}
	if len(command.Snapshot) == 0 || !json.Valid(command.Snapshot) {
		return ports.ErrInvalidRevisionRepository
	}
	dialect := db.Dialector.Name()
	revisionID, err := databaseid.Encode(dialect, command.Revision.ID)
	if err != nil {
		return ports.ErrInvalidRevisionRepository
	}
	accountID, err := databaseid.Encode(dialect, command.AccountID)
	if err != nil {
		return ports.ErrInvalidRevisionRepository
	}
	contentID, err := databaseid.Encode(dialect, command.Revision.ContentID)
	if err != nil {
		return ports.ErrInvalidRevisionRepository
	}
	translationID, err := databaseid.Encode(dialect, command.Revision.TranslationID)
	if err != nil {
		return ports.ErrInvalidRevisionRepository
	}
	authorID, err := databaseid.Encode(dialect, command.Revision.AuthorID)
	if err != nil {
		return ports.ErrInvalidRevisionRepository
	}
	if createErr := db.Table("content_revisions").Create(map[string]any{
		"id": revisionID, "account_id": accountID, "content_id": contentID,
		"translation_id": translationID, "author_id": authorID,
		"snapshot_hash":  command.Revision.SnapshotHash,
		"change_summary": command.Revision.ChangeSummary,
		"source_version": command.Revision.SourceVersion,
		"snapshot":       command.Snapshot, "created_at": command.Revision.CreatedAt,
	}).Error; createErr != nil {
		return fmt.Errorf("create content revision: %w", createErr)
	}
	return nil
}

func (repository *RevisionRepository) List(
	ctx context.Context,
	query ports.RevisionListQuery,
) (ports.RevisionPage, error) {
	if repository == nil || repository.db == nil || ctx == nil {
		return ports.RevisionPage{}, ErrInvalidRevisionRepository
	}
	if err := ports.ValidateRevisionListQuery(query); err != nil {
		return ports.RevisionPage{}, err
	}
	dialect := repository.db.Dialector.Name()
	accountID, err := databaseid.Encode(dialect, query.AccountID)
	if err != nil {
		return ports.RevisionPage{}, ports.ErrInvalidRevisionQuery
	}
	contentID, err := databaseid.Encode(dialect, query.ContentID)
	if err != nil {
		return ports.RevisionPage{}, ports.ErrInvalidRevisionQuery
	}
	translationID, err := databaseid.Encode(dialect, query.TranslationID)
	if err != nil {
		return ports.RevisionPage{}, ports.ErrInvalidRevisionQuery
	}
	db, err := repository.connection(ctx)
	if err != nil {
		return ports.RevisionPage{}, err
	}
	db = db.WithContext(ctx).Table("content_revisions").Select(
		"id AS revision_id, account_id, content_id, translation_id, author_id, "+
			"snapshot_hash, change_summary, source_version, snapshot, created_at",
	).Where(
		"account_id = ? AND content_id = ? AND translation_id = ?",
		accountID, contentID, translationID,
	)
	if query.After != nil {
		cursorID, encodeErr := databaseid.Encode(dialect, query.After.ID)
		if encodeErr != nil {
			return ports.RevisionPage{}, ports.ErrInvalidRevisionQuery
		}
		db = db.Where(
			"(created_at < ? OR (created_at = ? AND id < ?))",
			query.After.CreatedAt, query.After.CreatedAt, cursorID,
		)
	}
	var records []revisionRecord
	if err := db.Order("created_at DESC, id DESC").Limit(query.Limit + 1).Scan(&records).Error; err != nil {
		return ports.RevisionPage{}, fmt.Errorf("list content revisions: %w", err)
	}
	page := ports.RevisionPage{Items: make([]ports.RevisionView, 0, query.Limit)}
	if len(records) > query.Limit {
		records = records[:query.Limit]
		last := records[len(records)-1]
		page.Next = &ports.RevisionCursor{CreatedAt: last.CreatedAt.UTC(), ID: last.RevisionID.String()}
	}
	for _, record := range records {
		view, mapErr := mapRevisionRecord(record)
		if mapErr != nil {
			return ports.RevisionPage{}, mapErr
		}
		page.Items = append(page.Items, view)
	}
	return page, nil
}

func (repository *RevisionRepository) Get(
	ctx context.Context,
	accountID string,
	contentID string,
	revisionID string,
) (ports.RevisionView, error) {
	if repository == nil || repository.db == nil || ctx == nil {
		return ports.RevisionView{}, ErrInvalidRevisionRepository
	}
	dialect := repository.db.Dialector.Name()
	encodedAccountID, err := databaseid.Encode(dialect, accountID)
	if err != nil {
		return ports.RevisionView{}, ports.ErrRevisionNotFound
	}
	encodedContentID, err := databaseid.Encode(dialect, contentID)
	if err != nil {
		return ports.RevisionView{}, ports.ErrRevisionNotFound
	}
	encodedRevisionID, err := databaseid.Encode(dialect, revisionID)
	if err != nil {
		return ports.RevisionView{}, ports.ErrRevisionNotFound
	}
	db, err := repository.connection(ctx)
	if err != nil {
		return ports.RevisionView{}, err
	}
	var record revisionRecord
	result := db.WithContext(ctx).Table("content_revisions").Select(
		"id AS revision_id, account_id, content_id, translation_id, author_id, "+
			"snapshot_hash, change_summary, source_version, snapshot, created_at",
	).Where(
		"account_id = ? AND content_id = ? AND id = ?",
		encodedAccountID, encodedContentID, encodedRevisionID,
	).Limit(1).Scan(&record)
	if result.Error != nil {
		return ports.RevisionView{}, fmt.Errorf("get content revision: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return ports.RevisionView{}, ports.ErrRevisionNotFound
	}
	return mapRevisionRecord(record)
}

func (repository *RevisionRepository) RestoreContent(
	ctx context.Context,
	command ports.RestoreContentCommand,
) (ports.ContentDetail, error) {
	db, err := repository.connection(ctx)
	if err != nil {
		return ports.ContentDetail{}, err
	}
	dialect := db.Dialector.Name()
	accountID, err := databaseid.Encode(dialect, command.AccountID)
	if err != nil {
		return ports.ContentDetail{}, ports.ErrContentNotFound
	}
	contentID, err := databaseid.Encode(dialect, command.ContentID)
	if err != nil {
		return ports.ContentDetail{}, ports.ErrContentNotFound
	}
	translationID, err := databaseid.Encode(dialect, command.TranslationID)
	if err != nil {
		return ports.ContentDetail{}, ports.ErrContentNotFound
	}
	localeID, err := databaseid.Encode(dialect, command.Detail.LocaleID)
	if err != nil {
		return ports.ContentDetail{}, ports.ErrContentNotFound
	}
	blocks, err := json.Marshal(command.Detail.Blocks)
	if err != nil {
		return ports.ContentDetail{}, ports.ErrInvalidRevisionRepository
	}
	updatedAt := command.UpdatedAt.UTC()
	if updatedAt.IsZero() {
		return ports.ContentDetail{}, ports.ErrInvalidRevisionRepository
	}
	publishedAt := any(nil)
	if command.Detail.TranslationStatus == publishingdomain.StatusPublished {
		publishedAt = updatedAt
	}
	translationResult := db.Table("content_translations").Where(
		"account_id = ? AND content_id = ? AND id = ? AND version = ? AND deleted_at IS NULL",
		accountID, contentID, translationID, command.ExpectedVersion,
	).Updates(map[string]any{
		"locale_id": localeID, "slug": command.Detail.Slug, "path": command.Detail.Path,
		"title": command.Detail.Title, "excerpt": nullableString(command.Detail.Excerpt),
		"blocks": blocks, "seo_title": nullableString(command.Detail.SEO.Title),
		"seo_description": nullableString(command.Detail.SEO.Description),
		"geo_region":      nullableString(command.Detail.GEO.Region),
		"geo_locality":    nullableString(command.Detail.GEO.Locality),
		"aeo_question":    nullableString(command.Detail.AEO.Question),
		"aeo_answer":      nullableString(command.Detail.AEO.Answer),
		"status":          command.Detail.TranslationStatus, "published_at": publishedAt,
		"version": command.ExpectedVersion + 1, "updated_at": updatedAt,
	})
	if translationResult.Error != nil {
		return ports.ContentDetail{}, fmt.Errorf("restore content translation: %w", translationResult.Error)
	}
	if translationResult.RowsAffected != 1 {
		return ports.ContentDetail{}, ports.ErrRevisionConflict
	}
	itemResult := db.Table("content_items").Where(
		"account_id = ? AND id = ? AND deleted_at IS NULL", accountID, contentID,
	).Updates(map[string]any{
		"status": command.Detail.ContentStatus, "version": gormdb.Expr("version + ?", 1),
		"updated_at": updatedAt,
	})
	if itemResult.Error != nil {
		return ports.ContentDetail{}, fmt.Errorf("restore content item: %w", itemResult.Error)
	}
	if itemResult.RowsAffected != 1 {
		return ports.ContentDetail{}, ports.ErrContentNotFound
	}
	command.Detail.ContentVersion++
	command.Detail.TranslationVersion = command.ExpectedVersion + 1
	command.Detail.UpdatedAt = updatedAt
	return command.Detail, nil
}

func mapRevisionRecord(record revisionRecord) (ports.RevisionView, error) {
	if record.RevisionID.String() == "" || record.AccountID.String() == "" ||
		record.ContentID.String() == "" || record.TranslationID.String() == "" ||
		record.AuthorID.String() == "" || len(record.Snapshot) == 0 ||
		len(record.SnapshotHash) != 64 || record.SourceVersion < 1 || record.CreatedAt.IsZero() ||
		!json.Valid(record.Snapshot) {
		return ports.RevisionView{}, ErrInvalidRevisionRepository
	}
	revision, err := publishingdomain.NewRevision(publishingdomain.RevisionInput{
		ID: record.RevisionID.String(), ContentID: record.ContentID.String(),
		TranslationID: record.TranslationID.String(), AuthorID: record.AuthorID.String(),
		SnapshotHash: record.SnapshotHash, ChangeSummary: record.ChangeSummary,
		SourceVersion: record.SourceVersion, CreatedAt: record.CreatedAt,
	})
	if err != nil {
		return ports.RevisionView{}, ErrInvalidRevisionRepository
	}
	return ports.RevisionView{
		AccountID: record.AccountID.String(), Revision: revision,
		Snapshot: json.RawMessage(append([]byte(nil), record.Snapshot...)),
	}, nil
}

var _ ports.RevisionRepository = (*RevisionRepository)(nil)
